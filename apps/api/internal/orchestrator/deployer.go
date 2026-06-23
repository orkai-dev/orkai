package orchestrator

import (
	"context"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// Deployer handles application workload lifecycle.
type Deployer interface {
	Deploy(ctx context.Context, app *model.Application, opts DeployOpts) error
	Rollback(ctx context.Context, app *model.Application, revision int64) error
	Scale(ctx context.Context, app *model.Application, replicas int32) error
	UpdateEnvVars(ctx context.Context, app *model.Application, envVars map[string]string) error
	Restart(ctx context.Context, app *model.Application) error
	Stop(ctx context.Context, app *model.Application) error
	Delete(ctx context.Context, app *model.Application) error
	GetStatus(ctx context.Context, app *model.Application) (*AppStatus, error)
	GetPods(ctx context.Context, app *model.Application) ([]PodInfo, error)
	DeletePod(ctx context.Context, podName string, app *model.Application) error
	GetPodEvents(ctx context.Context, app *model.Application, podName string) ([]PodEvent, error)
	ConfigureHPA(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error
	DeleteHPA(ctx context.Context, app *model.Application) error
}

// AppManager is the legacy name for Deployer.
type AppManager = Deployer
