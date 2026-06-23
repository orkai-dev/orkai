package k3s

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/orkai-dev/orkai/apps/api/internal/config"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// BootstrapTargetRegistry loads the default deploy target from the store and
// wires the appropriate runtime implementation (K3s or noop).
func BootstrapTargetRegistry(
	ctx context.Context,
	s store.Store,
	cfg config.K8sConfig,
	logger *slog.Logger,
) (*orchestrator.TargetRegistry, error) {
	rec, err := s.DeployTargets().GetDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("load default deploy target: %w", err)
	}

	var target orchestrator.DeployTarget
	if cfg.InCluster || cfg.Kubeconfig != "" {
		orch, kerr := New(cfg, logger)
		if kerr != nil {
			logger.Warn("K3s connection failed, falling back to noop", slog.Any("error", kerr))
			target = orchestrator.NewNoopTarget(rec.ID, logger)
		} else {
			orch.defaultStorageClass = rec.Config.DefaultStorageClass
			target = NewTarget(orch, rec.ID)
		}
	} else {
		logger.Info("no KUBECONFIG set, using noop deploy target")
		target = orchestrator.NewNoopTarget(rec.ID, logger)
	}

	return orchestrator.NewTargetRegistry(rec.ID, target)
}
