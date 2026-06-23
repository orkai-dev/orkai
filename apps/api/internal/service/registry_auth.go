package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// ecrRefreshInterval controls how often short-lived registry pull secrets are
// refreshed. ECR tokens expire after ~12h, so refreshing every 6h keeps
// already-running apps pullable across pod reschedules.
const ecrRefreshInterval = 6 * time.Hour

// RegistryAuth resolves stored registry credentials into a Kubernetes
// dockerconfigjson document used as an imagePullSecret.
type RegistryAuth struct {
	store     store.Store
	targets   *orchestrator.TargetRegistry
	providers *providers.Registry
	logger    *slog.Logger
}

func NewRegistryAuth(s store.Store, targets *orchestrator.TargetRegistry, prov *providers.Registry, logger *slog.Logger) *RegistryAuth {
	return &RegistryAuth{store: s, targets: targets, providers: prov, logger: logger}
}

// StartECRRefresh launches a background loop that periodically re-applies
// short-lived registry pull secrets so long-running apps survive token expiry.
// When isLeader is non-nil, only the elected leader refreshes secrets.
func (r *RegistryAuth) StartECRRefresh(ctx context.Context, isLeader func() bool) {
	go func() {
		ticker := time.NewTicker(ecrRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if shouldRunSingleton(isLeader) {
					r.refreshECRSecrets(ctx)
				}
			}
		}
	}()
}

// refreshECRSecrets re-applies the dockerconfigjson pull secret for every app
// linked to a registry whose tokens are short-lived (e.g. ECR).
func (r *RegistryAuth) refreshECRSecrets(ctx context.Context) {
	apps, err := r.store.Applications().ListWithRegistry(ctx)
	if err != nil {
		r.logger.Error("ecr refresh: failed to list apps", slog.Any("error", err))
		return
	}
	for i := range apps {
		app := &apps[i]
		if app.RegistryID == nil {
			continue
		}
		res, err := r.store.SharedResources().GetByID(ctx, *app.RegistryID)
		if err != nil || !r.providers.Registry(res.Provider).ShortLived() {
			continue
		}
		cfg, err := r.DockerConfigJSON(ctx, *app.RegistryID)
		if err != nil {
			r.logger.Error("ecr refresh: failed to resolve token",
				slog.String("app", app.Name), slog.Any("error", err))
			continue
		}
		secrets, err := targetSecrets(r.targets, app)
		if err != nil {
			r.logger.Error("ecr refresh: deploy target missing secrets capability",
				slog.String("app", app.Name), slog.Any("error", err))
			continue
		}
		if err := secrets.EnsureImagePullSecret(ctx, app, cfg); err != nil {
			r.logger.Error("ecr refresh: failed to apply pull secret",
				slog.String("app", app.Name), slog.Any("error", err))
			continue
		}
		r.logger.Info("ecr refresh: pull secret refreshed", slog.String("app", app.Name))
	}
}

// DockerConfigJSONForApp resolves the dockerconfigjson for an app's linked
// registry. It returns (nil, nil) when the app has no registry configured.
func (r *RegistryAuth) DockerConfigJSONForApp(ctx context.Context, app *model.Application) ([]byte, error) {
	if app.RegistryID == nil {
		return nil, nil
	}
	return r.DockerConfigJSON(ctx, *app.RegistryID)
}

// dockerConfigJSON is the on-disk shape of a Docker config / Kubernetes
// .dockerconfigjson secret.
type dockerConfigJSON struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth"`
}

// DockerConfigJSON loads the registry SharedResource and returns a
// dockerconfigjson document for it. The registry provider resolves the host and
// credentials (e.g. ECR fetches a short-lived token; basic-auth providers use
// stored credentials).
func (r *RegistryAuth) DockerConfigJSON(ctx context.Context, registryID uuid.UUID) ([]byte, error) {
	res, err := r.store.SharedResources().GetByID(ctx, registryID)
	if err != nil {
		return nil, fmt.Errorf("registry not found: %w", err)
	}
	if res.Type != model.ResourceRegistry {
		return nil, fmt.Errorf("resource %s is not a registry", registryID)
	}

	host, username, password, err := r.providers.Registry(res.Provider).DockerAuth(ctx, res.Config)
	if err != nil {
		return nil, err
	}
	return buildDockerConfigJSON(host, username, password)
}

func buildDockerConfigJSON(host, username, password string) ([]byte, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	doc := dockerConfigJSON{
		Auths: map[string]dockerAuthEntry{
			host: {
				Username: username,
				Password: password,
				Auth:     auth,
			},
		},
	}
	return json.Marshal(doc)
}
