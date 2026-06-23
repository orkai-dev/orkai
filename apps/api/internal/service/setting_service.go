package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type SettingService struct {
	store   store.Store
	targets *orchestrator.TargetRegistry
	logger  *slog.Logger
}

func NewSettingService(s store.Store, targets *orchestrator.TargetRegistry, logger *slog.Logger) *SettingService {
	return &SettingService{store: s, targets: targets, logger: logger}
}

// InitDefaults runs on server startup. Detects the server IP and sets a default
// base domain. An explicit SERVER_IP env var (the public IP, which the installer
// knows) always wins and is re-applied on every startup, so it also corrects
// existing installs that previously auto-detected a private node IP.
func (s *SettingService) InitDefaults(ctx context.Context) error {
	envIP := strings.TrimSpace(os.Getenv("SERVER_IP"))
	if envIP != "" {
		// Authoritative override, reapplied each startup.
		if cur, _ := s.store.Settings().Get(ctx, model.SettingServerIP); cur != envIP {
			_ = s.store.Settings().Set(ctx, model.SettingServerIP, envIP)
			s.logger.Info("server IP set from SERVER_IP env", slog.String("ip", envIP))
		}
	}

	done, _ := s.store.Settings().Get(ctx, model.SettingSetupDone)
	if done == "true" {
		return nil
	}

	// First-run defaults: prefer the explicit env IP, then the K3s node IP, then
	// local network detection. On cloud VMs the node IP is private, so SERVER_IP
	// should carry the public IP.
	ip := envIP
	if ip == "" {
		ip = s.detectK3sNodeIP(ctx)
		if ip == "" {
			ip = detectLocalIP()
		}
		if ip != "" {
			_ = s.store.Settings().Set(ctx, model.SettingServerIP, ip)
		}
	}

	if ip != "" {
		s.logger.Info("detected server IP", slog.String("ip", ip))
		existing, _ := s.store.Settings().Get(ctx, model.SettingBaseDomain)
		if existing == "" {
			baseDomain := fmt.Sprintf("%s.sslip.io", ip)
			_ = s.store.Settings().Set(ctx, model.SettingBaseDomain, baseDomain)
			s.logger.Info("set default base domain", slog.String("domain", baseDomain))
		}
	}

	_ = s.store.Settings().Set(ctx, model.SettingSetupDone, "true")
	return nil
}

// ReconcileInfra re-applies infrastructure state on every startup.
// This ensures panel ingress, HTTPS redirect middleware, and other K8s
// resources survive restarts, accidental deletions, or cleanup operations.
func (s *SettingService) ReconcileInfra(ctx context.Context) {
	// Re-apply panel ingress if a domain is configured
	if err := s.applyPanelDomain(ctx, s.getPanelDomain(ctx)); err != nil {
		s.logger.Warn("reconcile: panel ingress not applied", slog.Any("error", err))
	}
}

func (s *SettingService) getPanelDomain(ctx context.Context) string {
	val, _ := s.store.Settings().Get(ctx, model.SettingPanelDomain)
	return val
}

// detectK3sNodeIP gets the InternalIP of the first control-plane node.
func (s *SettingService) detectK3sNodeIP(ctx context.Context) string {
	k8s, err := defaultK8s(s.targets)
	if err != nil {
		return ""
	}
	nodes, err := k8s.GetNodes(ctx)
	if err != nil || len(nodes) == 0 {
		return ""
	}

	// Prefer control-plane node IP
	for _, node := range nodes {
		if node.IP != "" {
			for _, role := range node.Roles {
				if role == "control-plane" || role == "master" {
					return node.IP
				}
			}
		}
	}

	// Fallback: any node with an IP
	for _, node := range nodes {
		if node.IP != "" {
			return node.IP
		}
	}

	return ""
}

// GetBaseDomain returns the configured base domain.
func (s *SettingService) GetBaseDomain(ctx context.Context) string {
	val, _ := s.store.Settings().Get(ctx, model.SettingBaseDomain)
	return val
}

func (s *SettingService) GetServerIP(ctx context.Context) string {
	val, _ := s.store.Settings().Get(ctx, "server_ip")
	return val
}

func (s *SettingService) GetAll(ctx context.Context) ([]model.Setting, error) {
	return s.store.Settings().GetAll(ctx)
}

func (s *SettingService) Get(ctx context.Context, key string) (string, error) {
	return s.store.Settings().Get(ctx, key)
}

func (s *SettingService) Set(ctx context.Context, key, value string) error {
	value = strings.TrimSpace(value)

	// A masked secret coming back from the client means "unchanged" — keep the
	// stored value rather than overwriting it with the placeholder.
	if value == model.SettingSecretMask && model.IsSensitiveSettingKey(key) {
		return nil
	}

	if key == model.SettingAuthGoogleOnly && value == "true" {
		if err := validateGoogleOAuthConfigured(ctx, s.store.Settings()); err != nil {
			return err
		}
	}

	if err := s.rejectGoogleOAuthChangeUnderGoogleOnly(ctx, key, value); err != nil {
		return err
	}

	if err := s.store.Settings().Set(ctx, key, value); err != nil {
		return err
	}
	s.logger.Info("setting updated", slog.String("key", key))

	// Apply side effects — setting is saved, but warn caller if infra failed
	switch key {
	case model.SettingPanelDomain:
		if err := s.applyPanelDomain(ctx, value); err != nil {
			s.logger.Warn("panel domain saved but ingress not applied", slog.Any("error", err))
			return fmt.Errorf("setting saved, but ingress not applied: %w", err)
		}
	case model.SettingHTTPSEmail:
		if err := s.applyHTTPSEmail(ctx, value); err != nil {
			s.logger.Warn("HTTPS email saved but not applied", slog.Any("error", err))
			return fmt.Errorf("setting saved, but HTTPS config not applied: %w", err)
		}
	}

	return nil
}

func validateGoogleOAuthConfigured(ctx context.Context, settings store.SettingStore) error {
	enabled, err := settings.Get(ctx, model.SettingGoogleOAuthEnabled)
	if err != nil {
		return fmt.Errorf("check google oauth enabled: %w", err)
	}
	if enabled != "true" {
		return fmt.Errorf("Google OAuth must be enabled before enforcing Google-only sign-in")
	}
	clientID, err := settings.Get(ctx, model.SettingGoogleOAuthClientID)
	if err != nil {
		return fmt.Errorf("check google oauth client id: %w", err)
	}
	clientSecret, err := settings.Get(ctx, model.SettingGoogleOAuthClientSecret)
	if err != nil {
		return fmt.Errorf("check google oauth client secret: %w", err)
	}
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("Google OAuth client ID and secret must be configured before enforcing Google-only sign-in")
	}
	return nil
}

func (s *SettingService) rejectGoogleOAuthChangeUnderGoogleOnly(ctx context.Context, key, value string) error {
	switch key {
	case model.SettingGoogleOAuthEnabled:
		if value == "true" {
			return nil
		}
	case model.SettingGoogleOAuthClientID, model.SettingGoogleOAuthClientSecret:
		if value != "" {
			return nil
		}
	default:
		return nil
	}

	googleOnly, err := s.store.Settings().Get(ctx, model.SettingAuthGoogleOnly)
	if err != nil {
		return fmt.Errorf("check google-only mode: %w", err)
	}
	if googleOnly == "true" {
		return fmt.Errorf("disable Google-only sign-in before changing Google OAuth settings")
	}
	return nil
}

// applyPanelDomain creates or removes the Traefik IngressRoute for the panel.
func (s *SettingService) applyPanelDomain(ctx context.Context, domain string) error {
	ingress, err := defaultIngress(s.targets)
	if err != nil {
		return err
	}
	if domain == "" {
		return ingress.DeletePanelIngress(ctx)
	}
	httpsEmail, _ := s.store.Settings().Get(ctx, model.SettingHTTPSEmail)
	return ingress.EnsurePanelIngress(ctx, domain, httpsEmail)
}

// applyHTTPSEmail updates the Traefik ACME certificate resolver email.
func (s *SettingService) applyHTTPSEmail(ctx context.Context, email string) error {
	panelDomain, _ := s.store.Settings().Get(ctx, model.SettingPanelDomain)
	if panelDomain != "" {
		ingress, err := defaultIngress(s.targets)
		if err != nil {
			return err
		}
		return ingress.EnsurePanelIngress(ctx, panelDomain, email)
	}
	return nil
}

// SMTPConfig holds SMTP mail server settings.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	Enabled  bool   `json:"enabled"`
}

// GetSMTPConfig reads SMTP settings from the settings table.
func (s *SettingService) GetSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	host, _ := s.store.Settings().Get(ctx, "smtp_host")
	port, _ := s.store.Settings().Get(ctx, "smtp_port")
	user, _ := s.store.Settings().Get(ctx, "smtp_user")
	password, _ := s.store.Settings().Get(ctx, "smtp_password")
	from, _ := s.store.Settings().Get(ctx, "smtp_from")
	enabled, _ := s.store.Settings().Get(ctx, "smtp_enabled")

	return &SMTPConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		From:     from,
		Enabled:  enabled == "true",
	}, nil
}

// SaveSMTPConfig writes SMTP settings to the settings table.
func (s *SettingService) SaveSMTPConfig(ctx context.Context, cfg *SMTPConfig) error {
	// Validate required fields when enabling
	if cfg.Enabled {
		if cfg.Host == "" {
			return fmt.Errorf("SMTP host is required")
		}
		if cfg.Port == "" {
			return fmt.Errorf("SMTP port is required")
		}
		if cfg.From == "" {
			return fmt.Errorf("SMTP from address is required")
		}
	}

	// If password is the masked placeholder, keep existing password
	if cfg.Password == model.SettingSecretMask {
		existing, err := s.GetSMTPConfig(ctx)
		if err == nil && existing.Password != "" {
			cfg.Password = existing.Password
		} else {
			cfg.Password = ""
		}
	}

	pairs := map[string]string{
		"smtp_host":     cfg.Host,
		"smtp_port":     cfg.Port,
		"smtp_user":     cfg.User,
		"smtp_password": cfg.Password,
		"smtp_from":     cfg.From,
	}
	if cfg.Enabled {
		pairs["smtp_enabled"] = "true"
	} else {
		pairs["smtp_enabled"] = "false"
	}
	for k, v := range pairs {
		if err := s.store.Settings().Set(ctx, k, v); err != nil {
			return err
		}
	}
	s.logger.Info("SMTP config updated")
	return nil
}

// detectLocalIP finds the primary non-loopback IPv4 address.
func detectLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				if !strings.HasPrefix(ip, "172.") && !strings.HasPrefix(ip, "169.254.") {
					return ip
				}
			}
		}
	}
	return "127.0.0.1"
}
