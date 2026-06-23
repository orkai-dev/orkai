package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type APIKeyService struct {
	store    store.Store
	logger   *slog.Logger
	notifSvc *NotificationService
}

func NewAPIKeyService(s store.Store, logger *slog.Logger, notifSvc *NotificationService) *APIKeyService {
	return &APIKeyService{store: s, logger: logger, notifSvc: notifSvc}
}

type CreateAPIKeyResult struct {
	Key    string       `json:"key"`
	APIKey model.APIKey `json:"api_key"`
}

func (s *APIKeyService) Create(
	ctx context.Context,
	userID, orgID uuid.UUID,
	callerRole string,
	name string,
	requestedRole model.Role,
	expiresAt *time.Time,
) (*CreateAPIKeyResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apierr.ErrValidation.WithDetail("name is required")
	}
	if requestedRole != model.RoleAdmin && requestedRole != model.RoleMember {
		return nil, apierr.ErrValidation.WithDetail("role must be 'admin' or 'member'")
	}
	if err := validateAPIKeyRole(callerRole, requestedRole); err != nil {
		return nil, err
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, apierr.ErrValidation.WithDetail("expires_at must be in the future")
	}

	raw, prefix, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	key := &model.APIKey{
		OrgID:     orgID,
		UserID:    userID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		Role:      requestedRole,
		ExpiresAt: expiresAt,
	}
	if err := s.store.APIKeys().Create(ctx, key); err != nil {
		return nil, err
	}

	s.logger.Info("api key created",
		slog.String("user_id", userID.String()),
		slog.String("key_id", key.ID.String()),
		slog.String("role", string(requestedRole)),
	)

	return &CreateAPIKeyResult{Key: raw, APIKey: *key}, nil
}

func validateAPIKeyRole(callerRole string, requestedRole model.Role) error {
	if callerRole == string(model.RoleAdmin) {
		return nil
	}
	if requestedRole == model.RoleMember {
		return nil
	}
	return apierr.ErrForbidden.WithDetail("members may only create member-scoped keys")
}

func (s *APIKeyService) List(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error) {
	return s.store.APIKeys().ListByUser(ctx, userID)
}

func (s *APIKeyService) Revoke(ctx context.Context, userID, keyID uuid.UUID) error {
	key, err := s.store.APIKeys().GetByIDForUser(ctx, keyID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apierr.ErrNotFound.WithDetail("api key not found")
		}
		return err
	}
	if err := s.store.APIKeys().Delete(ctx, keyID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apierr.ErrNotFound.WithDetail("api key not found")
		}
		return err
	}
	s.logger.Info("api key revoked",
		slog.String("user_id", userID.String()),
		slog.String("key_id", keyID.String()),
	)
	s.notifSvc.NotifyResourceDeleted(key.OrgID, model.EventAPIKeyRevoked,
		key.Name, fmt.Sprintf("API key %q was revoked", key.Name))
	return nil
}

func (s *APIKeyService) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return s.store.APIKeys().RevokeAllForUser(ctx, userID)
}

// GetByHash loads an API key by its hash for authentication middleware.
func (s *APIKeyService) GetByHash(ctx context.Context, hash string) (*model.APIKey, error) {
	key, err := s.store.APIKeys().GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apierr.ErrUnauthorized.WithDetail("invalid api key")
		}
		return nil, err
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, apierr.ErrUnauthorized.WithDetail("api key expired")
	}
	return key, nil
}

func (s *APIKeyService) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	return s.store.APIKeys().TouchLastUsed(ctx, id)
}
