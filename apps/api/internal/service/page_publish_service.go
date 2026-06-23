package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// PagePublishService clones a Page's git repo and resolves the publish folder
// whose contents are handed to the provider for syncing. There is NO build step
// in the MVP — only clone + locate files.
type PagePublishService struct {
	store     store.Store
	providers *providers.Registry
	logger    *slog.Logger
}

func NewPagePublishService(s store.Store, prov *providers.Registry, logger *slog.Logger) *PagePublishService {
	return &PagePublishService{store: s, providers: prov, logger: logger}
}

// PublishSource is the result of cloning + resolving the publish folder.
type PublishSource struct {
	// FilesDir is the directory whose contents should be synced to the origin.
	FilesDir string
	// CommitSHA is the HEAD commit of the cloned branch.
	CommitSHA string
	// PublishPath is the normalized publish folder that was resolved (e.g. "."
	// for repo root). Recorded on the deployment so dedup can detect a
	// publish_path change that isn't accompanied by a new commit.
	PublishPath string
	// Cleanup removes the cloned working tree; always call it when done.
	Cleanup func()
}

// Prepare clones the repo and resolves the publish folder. onLog streams
// progress (token-sanitized). It fails fast — before any cloud work — if the
// publish folder is missing at the cloned commit.
func (s *PagePublishService) Prepare(ctx context.Context, page *model.Page, onLog func(string)) (*PublishSource, error) {
	if page.GitRepo == "" {
		return nil, fmt.Errorf("page has no git repository configured")
	}

	workdir, err := os.MkdirTemp("", "orkai-page-*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(workdir) }
	repoDir := filepath.Join(workdir, "repo")

	token, tokenErr := s.resolveGitToken(ctx, page)
	if tokenErr != nil && page.GitProviderID != nil {
		// A provider was explicitly configured but the token could not be
		// resolved; fail early with a clear message rather than a confusing git
		// auth error.
		cleanup()
		return nil, fmt.Errorf("resolve git token: %w", tokenErr)
	}

	// Authenticate without ever putting the token on git's argv (which is
	// world-readable via /proc/<pid>/cmdline for same-UID processes). The URL
	// carries only the username; git fetches the password through GIT_ASKPASS,
	// which reads it from a 0600 token file.
	cloneURL, gitEnv, err := s.gitAuth(workdir, page.GitRepo, token)
	if err != nil {
		cleanup()
		return nil, err
	}

	onLog(fmt.Sprintf("Cloning %s (branch %s)…", page.GitRepo, page.GitBranch))
	branch := page.GitBranch
	if branch == "" {
		branch = "main"
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--depth", "1", "--single-branch", cloneURL, repoDir)
	cmd.Env = gitEnv
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return nil, fmt.Errorf("git clone failed: %s", sanitize(string(out), token))
	}

	// Resolve the publish folder. publish_path "." (or empty) = repo root.
	publishPath := strings.TrimSpace(page.PublishPath)
	if publishPath == "" {
		publishPath = "."
	}
	// Reject path traversal so a publish_path can't escape the clone.
	clean := filepath.Clean(publishPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || filepath.IsAbs(clean) {
		cleanup()
		return nil, fmt.Errorf("invalid publish folder %q", page.PublishPath)
	}
	filesDir := filepath.Join(repoDir, clean)

	info, err := os.Stat(filesDir)
	if err != nil || !info.IsDir() {
		cleanup()
		return nil, fmt.Errorf("publish folder %q not found in repo at branch %s — fix the folder before deploying", page.PublishPath, branch)
	}

	commitSHA := s.headSHA(ctx, repoDir)
	onLog(fmt.Sprintf("Resolved publish folder %q at commit %s", clean, shortSHA(commitSHA)))

	return &PublishSource{FilesDir: filesDir, CommitSHA: commitSHA, PublishPath: clean, Cleanup: cleanup}, nil
}

func (s *PagePublishService) headSHA(ctx context.Context, repoDir string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ResolveGitToken resolves a git token for the page's repo (GitHub App
// installation token first, then a linked provider's stored token). Used by the
// build path to clone in-cluster. A nil error still requires a non-empty token;
// callers decide whether the absence of one is fatal.
func (s *PagePublishService) ResolveGitToken(ctx context.Context, page *model.Page) (string, error) {
	return s.resolveGitToken(ctx, page)
}

// resolveGitToken mirrors BuildService.resolveGitToken: prefer a fresh GitHub
// App installation token, then fall back to the linked provider's stored token.
//
// Each resolution step's failure reason is collected (rather than swallowed) so
// the returned error and a debug log explain *why* no token was found — the
// difference between "no GitHub App configured", "installation token denied",
// and "git provider resource missing a token" matters when diagnosing a clone
// auth failure. A nil error still requires a successful token; callers decide
// whether the absence of one is fatal (it is only when GitProviderID is set).
func (s *PagePublishService) resolveGitToken(ctx context.Context, page *model.Page) (string, error) {
	var reasons []string

	if gh, err := s.providers.Git("github"); err != nil {
		reasons = append(reasons, "github app unavailable: "+err.Error())
	} else if token, terr := gh.CloneToken(ctx, page.GitRepo, nil); terr != nil {
		reasons = append(reasons, "github app installation token: "+terr.Error())
	} else if token != "" {
		return token, nil
	} else {
		reasons = append(reasons, "github app: no installation token for this repository")
	}

	if page.GitProviderID != nil {
		resource, err := s.store.SharedResources().GetByID(ctx, *page.GitProviderID)
		if err != nil {
			reasons = append(reasons, "git provider resource: "+err.Error())
		} else {
			var cfg struct {
				Token string `json:"token"`
			}
			if jerr := json.Unmarshal(resource.Config, &cfg); jerr != nil {
				reasons = append(reasons, "git provider config parse: "+jerr.Error())
			} else if cfg.Token == "" {
				reasons = append(reasons, "git provider resource has no token configured")
			} else {
				return cfg.Token, nil
			}
		}
	} else {
		reasons = append(reasons, "no git provider linked to the page")
	}

	if s.logger != nil {
		s.logger.Debug("no git token resolved for page",
			slog.String("page", page.Name),
			slog.String("reasons", strings.Join(reasons, "; ")))
	}
	return "", fmt.Errorf("no git token available for page %s: %s", page.Name, strings.Join(reasons, "; "))
}

// gitAuth prepares credential-helper-based authentication for the clone. It
// returns the clone URL (with a username but NO secret) and the environment to
// run git with. The token is written to a 0600 file inside workdir and supplied
// to git via a GIT_ASKPASS helper script, so it never appears on git's argv.
//
// For non-https repos or an empty token (public repo) the URL and a prompt-
// disabled environment are returned unchanged.
func (s *PagePublishService) gitAuth(workdir, repo, token string) (string, []string, error) {
	// Never let git block on an interactive credential prompt.
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if token == "" || !strings.HasPrefix(repo, "https://") {
		return repo, env, nil
	}

	// URL gets only the username; the password comes from the askpass helper.
	cloneURL := strings.Replace(repo, "https://", "https://x-access-token@", 1)

	tokenFile := filepath.Join(workdir, ".git-token")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		return "", nil, fmt.Errorf("write git token: %w", err)
	}

	// The askpass helper prints the token to stdout when git asks for the
	// password. (Username is already in the URL, so git never asks for it.)
	askpass := filepath.Join(workdir, "git-askpass.sh")
	script := "#!/bin/sh\ncat " + shellQuote(tokenFile) + "\n"
	if err := os.WriteFile(askpass, []byte(script), 0o700); err != nil {
		return "", nil, fmt.Errorf("write git askpass helper: %w", err)
	}

	env = append(env, "GIT_ASKPASS="+askpass)
	return cloneURL, env, nil
}

// shellQuote single-quotes a path for safe interpolation into the askpass /bin/sh
// script. Temp paths never contain single quotes, but quote defensively.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func sanitize(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	if sha == "" {
		return "unknown"
	}
	return sha
}
