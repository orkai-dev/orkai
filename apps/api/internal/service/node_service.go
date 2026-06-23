package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type NodeService struct {
	store    store.Store
	targets  *orchestrator.TargetRegistry
	logger   *slog.Logger
	notifSvc *NotificationService

	// Active log streams for WebSocket broadcasting
	mu      sync.RWMutex
	streams map[uuid.UUID][]chan string
}

func NewNodeService(s store.Store, targets *orchestrator.TargetRegistry, logger *slog.Logger, notifSvc *NotificationService) *NodeService {
	return &NodeService{
		store:    s,
		targets:  targets,
		logger:   logger,
		notifSvc: notifSvc,
		streams:  make(map[uuid.UUID][]chan string),
	}
}

// SubscribeLogs returns a channel that receives log lines for a node initialization.
func (s *NodeService) SubscribeLogs(nodeID uuid.UUID) chan string {
	ch := make(chan string, 256)
	s.mu.Lock()
	s.streams[nodeID] = append(s.streams[nodeID], ch)
	s.mu.Unlock()
	return ch
}

// UnsubscribeLogs removes a log subscriber.
func (s *NodeService) UnsubscribeLogs(nodeID uuid.UUID, ch chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	streams := s.streams[nodeID]
	for i, c := range streams {
		if c == ch {
			s.streams[nodeID] = append(streams[:i], streams[i+1:]...)
			close(ch)
			break
		}
	}
}

func (s *NodeService) broadcast(nodeID uuid.UUID, line string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.streams[nodeID] {
		select {
		case ch <- line:
		default:
		}
	}
}

// PushLogLine delivers a line to all active log subscribers for a node.
func (s *NodeService) PushLogLine(nodeID uuid.UUID, line string) {
	s.broadcast(nodeID, line)
}

func (s *NodeService) List(ctx context.Context) ([]model.ServerNode, error) {
	return s.store.ServerNodes().List(ctx)
}

func (s *NodeService) GetByID(ctx context.Context, id uuid.UUID) (*model.ServerNode, error) {
	return s.store.ServerNodes().GetByID(ctx, id)
}

type CreateNodeInput struct {
	Name     string     `json:"name" binding:"required"`
	Host     string     `json:"host" binding:"required"`
	Port     int        `json:"port"`
	SSHUser  string     `json:"ssh_user"`
	AuthType string     `json:"auth_type"` // password | ssh_key
	SSHKeyID *uuid.UUID `json:"ssh_key_id"`
	Password string     `json:"password"`
	Role     string     `json:"role"` // worker | server
}

func (s *NodeService) Create(ctx context.Context, orgID uuid.UUID, input CreateNodeInput) (*model.ServerNode, error) {
	// Validate SSH key belongs to same org
	if input.SSHKeyID != nil {
		res, err := s.store.SharedResources().GetByID(ctx, *input.SSHKeyID)
		if err != nil {
			return nil, fmt.Errorf("SSH key not found: %w", err)
		}
		if res.OrgID != orgID {
			return nil, fmt.Errorf("SSH key does not belong to this organization")
		}
	}

	node := &model.ServerNode{
		Name:     input.Name,
		Host:     input.Host,
		Port:     input.Port,
		SSHUser:  input.SSHUser,
		AuthType: input.AuthType,
		SSHKeyID: input.SSHKeyID,
		Password: input.Password,
		Role:     input.Role,
		Status:   model.NodeStatusPending,
	}
	if node.Port == 0 {
		node.Port = 22
	}
	if node.SSHUser == "" {
		node.SSHUser = "root"
	}
	if node.Role == "" {
		node.Role = "worker"
	}
	if node.AuthType == "" {
		node.AuthType = "password"
	}

	if err := s.store.ServerNodes().Create(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *NodeService) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	node, err := s.store.ServerNodes().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.ServerNodes().Delete(ctx, id); err != nil {
		return err
	}

	s.notifSvc.NotifyResourceDeleted(orgID, model.EventNodeDeleted,
		node.Name, fmt.Sprintf("Server node %q (%s) was deleted", node.Name, node.Host))
	return nil
}

// Initialize starts the K3s installation on the node via SSH.
// Runs in a goroutine; progress is broadcast via SubscribeLogs.
func (s *NodeService) Initialize(ctx context.Context, nodeID uuid.UUID) error {
	node, err := s.store.ServerNodes().GetByID(ctx, nodeID)
	if err != nil {
		return err
	}

	// Update status
	_ = s.store.ServerNodes().UpdateStatus(ctx, nodeID, model.NodeStatusInitializing, "")

	go s.runInitialize(node)
	return nil
}

func (s *NodeService) runInitialize(node *model.ServerNode) {
	ctx := context.Background()
	nodeID := node.ID

	s.broadcast(nodeID, fmt.Sprintf("Connecting to %s@%s:%d...", node.SSHUser, node.Host, node.Port))

	// Build SSH config with TOFU (Trust On First Use) host key verification
	var recordedFingerprint string
	config := &ssh.ClientConfig{
		User: node.SSHUser,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := fingerprintSHA256(key)
			if node.HostKeyFingerprint == "" {
				// First connection: record fingerprint (TOFU)
				recordedFingerprint = fp
				s.broadcast(nodeID, fmt.Sprintf("Host key fingerprint (TOFU): %s", fp))
				return nil
			}
			// Subsequent connections: verify fingerprint matches
			if fp != node.HostKeyFingerprint {
				return fmt.Errorf("host key mismatch: expected %s, got %s — possible MITM attack", node.HostKeyFingerprint, fp)
			}
			return nil
		},
		Timeout: 30 * time.Second,
	}

	switch node.AuthType {
	case "password":
		config.Auth = []ssh.AuthMethod{ssh.Password(node.Password)}
	case "ssh_key":
		// Load SSH key from shared_resources
		if node.SSHKeyID != nil {
			resource, err := s.store.SharedResources().GetByID(ctx, *node.SSHKeyID)
			if err != nil {
				s.finishWithError(ctx, nodeID, "Failed to load SSH key: "+err.Error())
				return
			}
			// Parse private key from config JSON
			var keyConfig struct {
				PrivateKey string `json:"private_key"`
				Passphrase string `json:"passphrase"`
			}
			if err := json.Unmarshal(resource.Config, &keyConfig); err != nil {
				s.finishWithError(ctx, nodeID, "Invalid SSH key config: "+err.Error())
				return
			}
			var signer ssh.Signer
			if keyConfig.Passphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(keyConfig.PrivateKey), []byte(keyConfig.Passphrase))
			} else {
				signer, err = ssh.ParsePrivateKey([]byte(keyConfig.PrivateKey))
			}
			if err != nil {
				s.finishWithError(ctx, nodeID, "Failed to parse SSH key: "+err.Error())
				return
			}
			config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	}

	// Connect
	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		s.finishWithError(ctx, nodeID, "SSH connection failed: "+err.Error())
		return
	}
	defer func() { _ = client.Close() }()

	s.broadcast(nodeID, "Connected successfully.")

	// Persist TOFU fingerprint on first connection
	if recordedFingerprint != "" {
		node.HostKeyFingerprint = recordedFingerprint
		_ = s.store.ServerNodes().Update(ctx, node)
	}

	// Get K3s server URL and token
	serverIP, _ := s.getK3sServerInfo()
	k3sURL := fmt.Sprintf("https://%s:6443", serverIP)
	k3sToken := s.getK3sToken(ctx)

	if k3sToken == "" {
		s.finishWithError(ctx, nodeID, "K3s token not configured. Set it in Settings → k3s_token")
		return
	}

	s.broadcast(nodeID, fmt.Sprintf("K3s server: %s", k3sURL))

	// Snapshot existing K8s nodes BEFORE installing, so we can detect the new one after
	existingNodes := make(map[string]bool)
	if k8s, err := defaultK8s(s.targets); err == nil {
		if currentNodes, nerr := k8s.GetNodes(ctx); nerr == nil {
			for _, n := range currentNodes {
				existingNodes[n.Name] = true
			}
		}
	}

	// Build the install script
	role := "agent"
	if node.Role == "server" {
		role = "server --server " + k3sURL + " --flannel-backend=wireguard-native"
	}

	// Sanitize shell arguments
	shellQuote := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}

	installCmd := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC=%s K3S_URL=%s K3S_TOKEN=%s sh -s - 2>&1`,
		shellQuote(role), shellQuote(k3sURL), shellQuote(k3sToken),
	)

	s.broadcast(nodeID, "Installing K3s...")

	// Execute command and stream output
	session, err := client.NewSession()
	if err != nil {
		s.finishWithError(ctx, nodeID, "Failed to create SSH session: "+err.Error())
		return
	}
	defer func() { _ = session.Close() }()

	stdout, err := session.StdoutPipe()
	if err != nil {
		s.finishWithError(ctx, nodeID, "Failed to pipe stdout: "+err.Error())
		return
	}

	if err := session.Start(installCmd); err != nil {
		s.finishWithError(ctx, nodeID, "Failed to start install command: "+err.Error())
		return
	}

	// Stream output line by line
	buf := make([]byte, 4096)
	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					s.broadcast(nodeID, line)
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				s.broadcast(nodeID, "Read error: "+readErr.Error())
			}
			break
		}
	}

	if err := session.Wait(); err != nil {
		s.finishWithError(ctx, nodeID, "K3s installation failed: "+err.Error())
		return
	}

	s.broadcast(nodeID, "K3s installed. Waiting for node to join cluster...")

	// Wait for a new node to appear that wasn't in the pre-install snapshot (up to 120s)
	for i := 0; i < 24; i++ {
		time.Sleep(5 * time.Second)
		k8s, kerr := defaultK8s(s.targets)
		if kerr != nil {
			continue
		}
		liveNodes, err := k8s.GetNodes(ctx)
		if err != nil {
			continue
		}
		for _, n := range liveNodes {
			if !existingNodes[n.Name] || n.IP == node.Host {
				s.broadcast(nodeID, fmt.Sprintf("Node %s joined the cluster! Status: %s", n.Name, n.Status))
				_ = s.store.ServerNodes().UpdateStatus(ctx, nodeID, model.NodeStatusReady, "")
				node.K8sNodeName = n.Name
				node.Status = model.NodeStatusReady
				_ = s.store.ServerNodes().Update(ctx, node)
				return
			}
		}
		s.broadcast(nodeID, fmt.Sprintf("Waiting... (%ds)", (i+1)*5))
	}

	s.finishWithError(ctx, nodeID, "Timeout: node did not join cluster within 120 seconds")
}

func (s *NodeService) finishWithError(ctx context.Context, nodeID uuid.UUID, msg string) {
	s.broadcast(nodeID, "ERROR: "+msg)
	_ = s.store.ServerNodes().UpdateStatus(ctx, nodeID, model.NodeStatusError, msg)
}

func (s *NodeService) getK3sServerInfo() (string, string) {
	ctx := context.Background()
	k8s, err := defaultK8s(s.targets)
	if err != nil {
		return "127.0.0.1", ""
	}
	nodes, err := k8s.GetNodes(ctx)
	if err != nil || len(nodes) == 0 {
		return "127.0.0.1", ""
	}
	// Find control-plane node
	for _, n := range nodes {
		for _, role := range n.Roles {
			if role == "control-plane" || role == "master" {
				return n.IP, n.Name
			}
		}
	}
	return nodes[0].IP, nodes[0].Name
}

func (s *NodeService) getK3sToken(ctx context.Context) string {
	token, err := s.store.Settings().Get(ctx, "k3s_token")
	if err != nil || token == "" {
		s.logger.Warn("k3s_token not configured in settings — set it via Settings page or API")
		return ""
	}
	return token
}

// fingerprintSHA256 returns the SHA256 fingerprint of an SSH public key.
func fingerprintSHA256(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
}
