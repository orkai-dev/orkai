//go:build integration

package pg_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

func TestDeployTargetStoreDefault(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	rec, err := s.DeployTargets().GetDefault(ctx)
	require.NoError(t, err)
	assert.Equal(t, model.DefaultDeployTargetID, rec.ID)
	assert.Equal(t, model.DeployTargetK3s, rec.Kind)
	assert.True(t, rec.IsDefault)
	assert.NotEmpty(t, rec.Capabilities)
}

func TestDeployTargetStoreGetByID(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	rec, err := s.DeployTargets().GetByID(ctx, model.DefaultDeployTargetID)
	require.NoError(t, err)
	assert.Equal(t, model.DeployTargetK3s, rec.Kind)
}

func TestDeployTargetStoreList(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	recs, err := s.DeployTargets().List(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, recs)
}

func TestDeployTargetStoreCreate(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	rec := &model.DeployTarget{
		BaseModel:    model.BaseModel{ID: uuid.New()},
		Kind:         model.DeployTargetK3s,
		Region:       "eu-west-1",
		Capabilities: []string{"deploy", "logs"},
	}
	require.NoError(t, s.DeployTargets().Create(ctx, rec))

	got, err := s.DeployTargets().GetByID(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", got.Region)
}
