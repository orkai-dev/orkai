package service

import (
	"context"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// targetForApp resolves the deploy target for an application.
func targetForApp(reg *orchestrator.TargetRegistry, app *model.Application) (orchestrator.DeployTarget, error) {
	return reg.For(app)
}

// deployTargetRecord loads the persisted deploy target for an application.
func deployTargetRecord(ctx context.Context, s store.Store, app *model.Application) (*model.DeployTarget, error) {
	if app.TargetID != nil {
		return s.DeployTargets().GetByID(ctx, *app.TargetID)
	}
	return s.DeployTargets().GetDefault(ctx)
}

// defaultK8s returns the KubernetesInspector from the default deploy target.
func defaultK8s(reg *orchestrator.TargetRegistry) (orchestrator.KubernetesInspector, error) {
	return orchestrator.RequireKubernetesInspector(reg.Default())
}

// defaultIngress returns IngressBinder from the default deploy target (panel ingress).
func defaultIngress(reg *orchestrator.TargetRegistry) (orchestrator.IngressBinder, error) {
	return orchestrator.AsCapability[orchestrator.IngressBinder](reg.Default(), orchestrator.CapIngress)
}

// targetIngress resolves IngressBinder for an application.
func targetIngress(reg *orchestrator.TargetRegistry, app *model.Application) (orchestrator.IngressBinder, error) {
	t, err := reg.For(app)
	if err != nil {
		return nil, err
	}
	return orchestrator.AsCapability[orchestrator.IngressBinder](t, orchestrator.CapIngress)
}

// targetBuilder resolves Builder for an application.
func targetBuilder(reg *orchestrator.TargetRegistry, app *model.Application) (orchestrator.Builder, error) {
	t, err := reg.For(app)
	if err != nil {
		return nil, err
	}
	return orchestrator.AsCapability[orchestrator.Builder](t, orchestrator.CapBuild)
}

// targetSecrets resolves SecretSink for an application.
func targetSecrets(reg *orchestrator.TargetRegistry, app *model.Application) (orchestrator.SecretSink, error) {
	t, err := reg.For(app)
	if err != nil {
		return nil, err
	}
	return orchestrator.AsCapability[orchestrator.SecretSink](t, orchestrator.CapSecrets)
}

// targetCron resolves CronManager from the default target (cronjobs are cluster-scoped today).
func targetCron(reg *orchestrator.TargetRegistry) (orchestrator.CronManager, error) {
	return orchestrator.AsCapability[orchestrator.CronManager](reg.Default(), orchestrator.CapCron)
}

// targetDatabase resolves DatabaseManager from the default target.
func targetDatabase(reg *orchestrator.TargetRegistry) (orchestrator.DatabaseManager, error) {
	return orchestrator.AsCapability[orchestrator.DatabaseManager](reg.Default(), orchestrator.CapManagedDB)
}
