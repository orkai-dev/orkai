package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	appVersion "github.com/orkai-dev/orkai/apps/api/internal/version"
	"golang.org/x/sync/singleflight"
)

const upgradeStatusFile = "/opt/orkai/upgrade_status.json"

// versionInfoCacheTTL bounds how often non-leader replicas re-read the shared
// version_info setting. The leader refreshes at most hourly, so a short local
// TTL avoids a DB round-trip on every /version request.
const versionInfoCacheTTL = 5 * time.Minute

type VersionInfo struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	UpdateAvail bool   `json:"update_available"`
	ReleaseURL  string `json:"release_url"`
	Changelog   string `json:"changelog"`
	PublishedAt string `json:"published_at"`
}

type UpgradeStatus struct {
	Status  string `json:"status"` // idle, upgrading, done, error
	Message string `json:"message"`
}

type VersionService struct {
	store      store.Store
	logger     *slog.Logger
	mu         sync.RWMutex
	cached     *VersionInfo
	shared     *VersionInfo
	sharedExp  time.Time
	sharedLoad singleflight.Group
	upgradeMu  sync.Mutex
}

func NewVersionService(s store.Store, logger *slog.Logger) *VersionService {
	return &VersionService{store: s, logger: logger}
}

// Start begins periodic version checks against GitHub releases.
// When isLeader is non-nil, only the elected leader checks for updates.
func (s *VersionService) Start(ctx context.Context, isLeader func() bool) {
	go func() {
		if shouldRunSingleton(isLeader) {
			s.checkForUpdate()
		}
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if shouldRunSingleton(isLeader) {
					s.checkForUpdate()
				}
			}
		}
	}()
}

func (s *VersionService) GetVersionInfo(ctx context.Context) *VersionInfo {
	if info := s.getSharedVersionInfo(); info != nil {
		return info
	}

	if s.store != nil {
		v, _, _ := s.sharedLoad.Do("version_info", func() (any, error) {
			if info := s.getSharedVersionInfo(); info != nil {
				return info, nil
			}
			return s.loadSharedVersionInfo(ctx), nil
		})
		if info, ok := v.(*VersionInfo); ok && info != nil {
			return info
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cached != nil {
		return s.cached
	}

	return &VersionInfo{
		Current: appVersion.Version,
		Latest:  appVersion.Version,
	}
}

func (s *VersionService) getSharedVersionInfo() *VersionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.shared != nil && time.Now().Before(s.sharedExp) {
		return s.shared
	}
	return nil
}

func (s *VersionService) loadSharedVersionInfo(ctx context.Context) *VersionInfo {
	raw, err := s.store.Settings().Get(ctx, model.SettingVersionInfo)
	if err != nil || raw == "" {
		return nil
	}
	var info VersionInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil
	}
	s.mu.Lock()
	s.shared = &info
	s.sharedExp = time.Now().Add(versionInfoCacheTTL)
	s.mu.Unlock()
	return &info
}

func (s *VersionService) checkForUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		appVersion.GitHubOwner, appVersion.GitHubRepo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.Debug("version check: failed to create request", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := notifHTTPClient.Do(req)
	if err != nil {
		s.logger.Debug("version check: request failed", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		s.logger.Debug("version check: non-200 response", slog.Int("status", resp.StatusCode))
		s.setCached(appVersion.Version, "", false)
		return
	}

	var release struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		s.logger.Debug("version check: failed to decode", slog.Any("error", err))
		s.setCached(appVersion.Version, "", false)
		return
	}

	latest := release.TagName
	updateAvail := appVersion.Version != "dev" && shouldUpdate(latest, appVersion.Version)

	info := &VersionInfo{
		Current:     appVersion.Version,
		Latest:      latest,
		UpdateAvail: updateAvail,
		ReleaseURL:  release.HTMLURL,
		Changelog:   release.Body,
		PublishedAt: release.PublishedAt,
	}
	s.saveVersionInfo(ctx, info)

	if updateAvail {
		s.logger.Info("new version available", slog.String("current", appVersion.Version), slog.String("latest", latest))
	}
}

func shouldUpdate(latest, current string) bool {
	latestClean := strings.TrimPrefix(latest, "v")
	currentClean := strings.TrimPrefix(current, "v")

	if latestClean == currentClean {
		return false
	}

	if strings.Contains(currentClean, "-") {
		currentBase := currentClean[:strings.IndexByte(currentClean, '-')]
		latestBase := latestClean
		if idx := strings.IndexByte(latestBase, '-'); idx != -1 {
			latestBase = latestBase[:idx]
		}
		if currentBase == latestBase && !strings.Contains(latestClean, "-") {
			return true
		}
	}

	return isNewer(latest, current)
}

func isNewer(latest, current string) bool {
	parse := func(v string) (int, int, int) {
		v = strings.TrimPrefix(v, "v")
		if idx := strings.IndexByte(v, '-'); idx != -1 {
			v = v[:idx]
		}
		parts := strings.SplitN(v, ".", 3)
		a, _ := strconv.Atoi(parts[0])
		b, c := 0, 0
		if len(parts) > 1 {
			b, _ = strconv.Atoi(parts[1])
		}
		if len(parts) > 2 {
			c, _ = strconv.Atoi(parts[2])
		}
		return a, b, c
	}
	la, lb, lc := parse(latest)
	ca, cb, cc := parse(current)
	if la != ca {
		return la > ca
	}
	if lb != cb {
		return lb > cb
	}
	return lc > cc
}

// ============================================================================
// Upgrade — spawns a one-shot upgrader container that runs upgrade-lib.sh
// ============================================================================

// GetUpgradeStatus reads the shared status file written by the upgrader.
func (s *VersionService) GetUpgradeStatus() UpgradeStatus {
	data, err := os.ReadFile(upgradeStatusFile)
	if err != nil {
		return UpgradeStatus{Status: "idle"}
	}
	var st UpgradeStatus
	if err := json.Unmarshal(data, &st); err != nil {
		return UpgradeStatus{Status: "idle"}
	}
	return st
}

// ClearUpgradeStatus resets the status file after frontend acknowledges.
func (s *VersionService) ClearUpgradeStatus() {
	_ = os.Remove(upgradeStatusFile)
}

// TriggerUpgrade spawns a one-shot "upgrader" container that runs the shared
// upgrade-lib.sh script. The upgrader is an independent process — it survives
// the main orkai container being replaced, so it can do health checks and rollback.
func (s *VersionService) TriggerUpgrade() error {
	s.upgradeMu.Lock()
	defer s.upgradeMu.Unlock()

	// Check if upgrader container is actually running (not just status file)
	if out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", "orkai-upgrader").Output(); err == nil {
		if strings.TrimSpace(string(out)) == "true" {
			return fmt.Errorf("upgrade already in progress (upgrader container is running)")
		}
		// Container exists but not running — remove it
		_ = exec.Command("docker", "rm", "-f", "orkai-upgrader").Run()
	}

	// Also check status file as secondary guard
	current := s.GetUpgradeStatus()
	if current.Status == "upgrading" {
		return fmt.Errorf("upgrade already in progress")
	}

	// Preflight: verify docker.sock
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return fmt.Errorf("docker socket not mounted — add /var/run/docker.sock volume to the orkai container")
	}
	// Preflight: verify upgrade-lib.sh exists
	if _, err := os.Stat("/opt/orkai/upgrade-lib.sh"); err != nil {
		return fmt.Errorf("upgrade-lib.sh not found at /opt/orkai — ensure /opt/orkai is mounted")
	}

	// Write initial status
	if err := writeUpgradeStatus(UpgradeStatus{Status: "upgrading", Message: "Starting upgrade..."}); err != nil {
		return fmt.Errorf("cannot persist upgrade state: %w", err)
	}

	// Spawn upgrader container:
	// - docker:cli image has docker + docker compose + curl + sh
	// - Host network for healthz checks on localhost:3000
	// - Mounts docker.sock (to manage containers) and /opt/orkai (shared state + scripts)
	// - 10 minute timeout via --stop-timeout
	// - Runs the same upgrade-lib.sh that upgrade.sh uses
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", "orkai-upgrader",
		"--network", "host",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", "/opt/orkai:/opt/orkai",
		"docker:cli",
		"sh", "-c", ". /opt/orkai/upgrade-lib.sh && run_upgrade container",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("Failed to start upgrader: %s", strings.TrimSpace(string(out)))
		s.logger.Error("upgrade: "+msg, slog.Any("error", err))
		_ = writeUpgradeStatus(UpgradeStatus{Status: "error", Message: msg})
		return fmt.Errorf("%s", msg)
	}

	containerID := strings.TrimSpace(string(out))
	s.logger.Info("upgrade: upgrader container started", slog.String("container_id", containerID[:12]))
	return nil
}

func writeUpgradeStatus(st UpgradeStatus) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(upgradeStatusFile, data, 0o644)
}

func (s *VersionService) setCached(current, latest string, updateAvail bool) {
	// Transient GitHub failures only update the local cache so we do not
	// overwrite the last-known-good version_info row shared across replicas.
	s.mu.Lock()
	s.cached = &VersionInfo{
		Current:     current,
		Latest:      latest,
		UpdateAvail: updateAvail,
	}
	s.mu.Unlock()
}

func (s *VersionService) saveVersionInfo(ctx context.Context, info *VersionInfo) {
	s.mu.Lock()
	s.cached = info
	s.shared = info
	s.sharedExp = time.Now().Add(versionInfoCacheTTL)
	s.mu.Unlock()

	if s.store == nil {
		return
	}
	raw, err := json.Marshal(info)
	if err != nil {
		s.logger.Warn("version check: failed to marshal version info", slog.Any("error", err))
		return
	}
	if err := s.store.Settings().Set(ctx, model.SettingVersionInfo, string(raw)); err != nil {
		s.logger.Warn("version check: failed to persist version info", slog.Any("error", err))
	}
}
