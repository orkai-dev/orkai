package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentIsValid(t *testing.T) {
	for _, env := range ValidEnvironments {
		assert.True(t, env.IsValid(), "%s should be valid", env)
	}
	assert.False(t, Environment("bogus").IsValid())
	assert.False(t, Environment("").IsValid())
}

func TestProjectAfterScanRowInitializesNils(t *testing.T) {
	p := &Project{}
	require.NoError(t, p.AfterScanRow(context.Background()))
	assert.NotNil(t, p.EnvVars)
	assert.NotNil(t, p.ResourceQuota)
}

func TestProjectAfterScanRowKeepsExisting(t *testing.T) {
	p := &Project{
		EnvVars:       map[string]string{"A": "1"},
		ResourceQuota: &ResourceQuotaConfig{CPULimit: "1000m"},
	}
	require.NoError(t, p.AfterScanRow(context.Background()))
	assert.Equal(t, "1", p.EnvVars["A"])
	assert.Equal(t, "1000m", p.ResourceQuota.CPULimit)
}

func TestApplicationAfterScanRowInitializesNils(t *testing.T) {
	a := &Application{}
	require.NoError(t, a.AfterScanRow(context.Background()))
	assert.NotNil(t, a.EnvVars)
	assert.NotNil(t, a.BuildArgs)
	assert.NotNil(t, a.Ports)
	assert.NotNil(t, a.WatchPaths)
	assert.NotNil(t, a.Volumes)
	assert.NotNil(t, a.BuildEnvVars)
	assert.NotNil(t, a.Secrets)
	assert.NotNil(t, a.HealthCheck)
	assert.NotNil(t, a.Autoscaling)
	assert.NotNil(t, a.DeployStrategyConfig)
}

func TestApplicationAfterScanRowKeepsExisting(t *testing.T) {
	a := &Application{
		EnvVars:              map[string]string{"A": "1"},
		BuildArgs:            map[string]string{"B": "2"},
		Ports:                []PortMapping{{}},
		WatchPaths:           []string{"x"},
		Volumes:              []VolumeMount{{}},
		BuildEnvVars:         map[string]string{"C": "3"},
		Secrets:              map[string]string{"D": "4"},
		HealthCheck:          &HealthCheck{},
		Autoscaling:          &AutoscalingConfig{},
		DeployStrategyConfig: &DeployStrategyConfig{},
	}
	require.NoError(t, a.AfterScanRow(context.Background()))
	assert.Equal(t, "1", a.EnvVars["A"])
	assert.Equal(t, "2", a.BuildArgs["B"])
	assert.Len(t, a.WatchPaths, 1)
}

func TestCronJobAfterScanRow(t *testing.T) {
	cj := &CronJob{}
	require.NoError(t, cj.AfterScanRow(context.Background()))
	assert.NotNil(t, cj.EnvVars)

	cj2 := &CronJob{EnvVars: map[string]string{"K": "V"}}
	require.NoError(t, cj2.AfterScanRow(context.Background()))
	assert.Equal(t, "V", cj2.EnvVars["K"])
}

func TestIsValidVersion(t *testing.T) {
	assert.True(t, IsValidVersion(DBPostgres, "18"))
	assert.True(t, IsValidVersion(DBMySQL, "8.0"))
	assert.True(t, IsValidVersion(DBValkey, "8.1"))
	assert.False(t, IsValidVersion(DBValkey, "1.0"))
	assert.False(t, IsValidVersion(DBPostgres, "99"))
	assert.False(t, IsValidVersion(DBEngine("unknown"), "1"))
}

func TestAllNotifyEvents(t *testing.T) {
	events := AllNotifyEvents()
	assert.NotEmpty(t, events)
	assert.Contains(t, events, EventDeploySuccess)
	assert.Contains(t, events, EventDatabaseDeleted)
	assert.Contains(t, events, EventAppDeleted)
	assert.Contains(t, events, EventProjectDeleted)
	assert.Contains(t, events, EventAPIKeyRevoked)
	assert.Contains(t, events, EventPVCDeleted)
}

func TestAllNotifyEventInfos(t *testing.T) {
	infos := AllNotifyEventInfos()
	assert.NotEmpty(t, infos)
	assert.Len(t, infos, len(AllNotifyEvents()))

	keys := make(map[NotifyEvent]struct{}, len(infos))
	for _, info := range infos {
		assert.NotEmpty(t, info.Label)
		assert.NotEmpty(t, info.Category)
		keys[info.Key] = struct{}{}
	}
	for _, event := range AllNotifyEvents() {
		assert.Contains(t, keys, event)
	}
}

func TestIsValidAvatar(t *testing.T) {
	assert.True(t, IsValidAvatar("bear"))
	assert.True(t, IsValidAvatar("wolf"))
	assert.False(t, IsValidAvatar("dragon"))
	assert.False(t, IsValidAvatar(""))
}
