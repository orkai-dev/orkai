package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// limitedTarget implements only Deployer for capability-gating tests.
type limitedTarget struct {
	orchestrator.Deployer
	id uuid.UUID
}

func (l *limitedTarget) ID() uuid.UUID { return l.id }
func (l *limitedTarget) Kind() string  { return "limited" }
func (l *limitedTarget) Capabilities() orchestrator.CapSet {
	return orchestrator.NewCapSet(orchestrator.CapDeploy)
}

func TestDefaultK8sUnsupported(t *testing.T) {
	limited := &limitedTarget{Deployer: orchestrator.NewNoop(nil), id: uuid.New()}
	reg, err := orchestrator.NewTargetRegistry(limited.ID(), limited)
	require.NoError(t, err)

	_, err = defaultK8s(reg)
	require.Error(t, err)
	var capErr orchestrator.ErrCapabilityUnsupported
	require.ErrorAs(t, err, &capErr)
	assert.Equal(t, orchestrator.CapKubernetes, capErr.Capability)
}

func TestAppServiceGetTargetCapabilities(t *testing.T) {
	fs := testsupport.NewFakeStore()
	appID := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: appID}}, nil
	}
	svc := newAppService(fs, testsupport.NewFakeOrchestrator())
	caps, err := svc.GetTargetCapabilities(context.Background(), appID)
	require.NoError(t, err)
	assert.Equal(t, "k3s", caps.Kind)
	assert.True(t, len(caps.Capabilities) > 0)
}

func TestAppServiceGetTargetCapabilitiesStorageConfig(t *testing.T) {
	fs := testsupport.NewFakeStore()
	appID := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: appID}}, nil
	}
	target, err := fs.DeployTargets().GetDefault(context.Background())
	require.NoError(t, err)
	target.Config.DefaultStorageClass = "gp3"
	target.Config.AllowedStorageClasses = []string{"gp3", "local-path"}
	require.NoError(t, fs.DeployTargets().Update(context.Background(), target))

	svc := newAppService(fs, testsupport.NewFakeOrchestrator())
	caps, err := svc.GetTargetCapabilities(context.Background(), appID)
	require.NoError(t, err)
	assert.Equal(t, "gp3", caps.DefaultStorageClass)
	assert.Equal(t, []string{"gp3", "local-path"}, caps.AllowedStorageClasses)
}
