package providers

import (
	"log/slog"
	"net/http"
	"time"
)

// Registry resolves provider implementations by their stored string key. It is
// constructed once and shared (read-only after construction) across services.
type Registry struct {
	git        map[string]GitProvider
	reg        map[string]RegistryProvider
	objStorage map[string]ObjectStorageProvider
}

// New builds a Registry wired with all built-in providers. settings is used by
// providers that need instance configuration (e.g. GitHub App credentials).
func New(settings SettingsGetter, logger *slog.Logger) *Registry {
	deps := factoryDeps{
		settings: settings,
		client:   &http.Client{Timeout: 15 * time.Second},
		logger:   logger,
	}

	git := make(map[string]GitProvider, len(gitFactoryFns))
	for _, fn := range gitFactoryFns {
		p := fn(deps)
		git[p.Name()] = p
	}

	reg := make(map[string]RegistryProvider, len(regFactoryFns))
	for _, fn := range regFactoryFns {
		p := fn(deps)
		reg[p.Name()] = p
	}

	objStorage := make(map[string]ObjectStorageProvider, len(objStorageFactoryFns))
	for _, fn := range objStorageFactoryFns {
		p := fn(deps)
		objStorage[p.Name()] = p
	}

	return &Registry{git: git, reg: reg, objStorage: objStorage}
}

// Git returns the git provider registered under name, or ErrUnsupportedProvider.
func (r *Registry) Git(name string) (GitProvider, error) {
	p, ok := r.git[name]
	if !ok {
		return nil, ErrUnsupportedProvider{Provider: name}
	}
	return p, nil
}

// Registry returns the container-registry provider registered under name.
// Unknown keys (including "") fall back to the basic-auth "custom" provider,
// preserving the legacy behavior where any non-ECR registry used basic auth.
func (r *Registry) Registry(name string) RegistryProvider {
	if p, ok := r.reg[name]; ok {
		return p
	}
	return r.reg["custom"]
}

// ObjectStorage returns the object-storage provider registered under name.
// Unknown keys fall back to the S3-compatible "aws_s3" implementation because
// every current object_storage provider key (minio, r2, …) uses the same API.
func (r *Registry) ObjectStorage(name string) ObjectStorageProvider {
	if p, ok := r.objStorage[name]; ok {
		return p
	}
	return r.objStorage["aws_s3"]
}
