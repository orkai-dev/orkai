package service

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAPIKeyRole(t *testing.T) {
	assert.NoError(t, validateAPIKeyRole(string(model.RoleAdmin), model.RoleAdmin))
	assert.NoError(t, validateAPIKeyRole(string(model.RoleAdmin), model.RoleMember))
	assert.NoError(t, validateAPIKeyRole(string(model.RoleMember), model.RoleMember))
	assert.Error(t, validateAPIKeyRole(string(model.RoleMember), model.RoleAdmin))
}

func TestAPIKeyServiceRevokeNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	keyID := uuid.New()
	userID := uuid.New()
	fs.APIKeysStore.GetByIDForUserFn = func(_ context.Context, id, uid uuid.UUID) (*model.APIKey, error) {
		if id == keyID && uid == userID {
			return nil, sql.ErrNoRows
		}
		return nil, sql.ErrNoRows
	}

	svc := NewAPIKeyService(fs, slog.Default(), nil)
	err := svc.Revoke(context.Background(), userID, keyID)
	require.Error(t, err)
	pd, ok := err.(*apierr.ProblemDetail)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, pd.Status)
	assert.Equal(t, "api key not found", pd.Detail)
}

func TestAPIKeyServiceGetByHash(t *testing.T) {
	fs := testsupport.NewFakeStore()
	keyID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()
	fs.APIKeysStore.GetByHashFn = func(_ context.Context, hash string) (*model.APIKey, error) {
		if hash == "known" {
			return &model.APIKey{
				BaseModel: model.BaseModel{ID: keyID},
				UserID:    userID,
				OrgID:     orgID,
				Role:      model.RoleAdmin,
			}, nil
		}
		return nil, sql.ErrNoRows
	}

	svc := NewAPIKeyService(fs, slog.Default(), nil)

	key, err := svc.GetByHash(context.Background(), "known")
	require.NoError(t, err)
	assert.Equal(t, keyID, key.ID)

	_, err = svc.GetByHash(context.Background(), "missing")
	require.Error(t, err)
	pd, ok := err.(*apierr.ProblemDetail)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, pd.Status)
	assert.Equal(t, "invalid api key", pd.Detail)
}

func TestAPIKeyServiceGetByHashExpired(t *testing.T) {
	fs := testsupport.NewFakeStore()
	expired := time.Now().Add(-time.Hour)
	fs.APIKeysStore.GetByHashFn = func(_ context.Context, hash string) (*model.APIKey, error) {
		return &model.APIKey{
			BaseModel: model.BaseModel{ID: uuid.New()},
			ExpiresAt: &expired,
		}, nil
	}

	svc := NewAPIKeyService(fs, slog.Default(), nil)
	_, err := svc.GetByHash(context.Background(), "known")
	require.Error(t, err)
	pd, ok := err.(*apierr.ProblemDetail)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, pd.Status)
	assert.Equal(t, "api key expired", pd.Detail)
}
