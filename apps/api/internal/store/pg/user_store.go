package pg

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type userStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *userStore) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user := new(model.User)
	err := s.db.NewSelect().Model(user).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptUser(user, s.secrets); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := new(model.User)
	err := s.db.NewSelect().Model(user).Where("email = ?", email).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptUser(user, s.secrets); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userStore) Create(ctx context.Context, user *model.User) error {
	encSecret, err := encryptUser2FA(s.secrets, user.TwoFASecret)
	if err != nil {
		return err
	}
	user.TwoFASecret = encSecret
	_, err = s.db.NewInsert().Model(user).Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptUser(user, s.secrets)
}

func (s *userStore) Update(ctx context.Context, user *model.User) error {
	encSecret, err := encryptUser2FA(s.secrets, user.TwoFASecret)
	if err != nil {
		return err
	}
	user.TwoFASecret = encSecret
	_, err = s.db.NewUpdate().Model(user).WherePK().Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptUser(user, s.secrets)
}

func (s *userStore) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("password_hash = ?", passwordHash).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (s *userStore) Update2FA(ctx context.Context, userID uuid.UUID, enabled bool, secretVal string) error {
	stored, err := encryptUser2FA(s.secrets, secretVal)
	if err != nil {
		return err
	}
	_, err = s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("two_fa_enabled = ?", enabled).
		Set("two_fa_secret = ?", stored).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (s *userStore) ListByOrg(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.User, int, error) {
	var users []model.User
	count, err := s.db.NewSelect().
		Model(&users).
		Where("org_id = ?", orgID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := decryptUsers(users, s.secrets); err != nil {
		return nil, 0, err
	}
	return users, count, nil
}

func (s *userStore) UpdateRole(ctx context.Context, userID uuid.UUID, role string) error {
	_, err := s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("role = ?", role).
		Set("token_version = token_version + 1").
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (s *userStore) RemoveFromOrg(ctx context.Context, userID uuid.UUID) error {
	// org_id is NOT NULL and every user belongs to exactly one org, so removing
	// a user from their org means deleting the account. Bump token_version so
	// JWTs invalidate immediately, then soft-delete via deleted_at.
	_, err := s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("token_version = token_version + 1").
		Set("deleted_at = ?", time.Now()).
		Where("id = ?", userID).
		Where("deleted_at IS NULL").
		Exec(ctx)
	return err
}

func (s *userStore) BumpTokenVersion(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("token_version = token_version + 1").
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (s *userStore) Count(ctx context.Context) (int, error) {
	return s.db.NewSelect().Model((*model.User)(nil)).Count(ctx)
}

func (s *userStore) CountByRole(ctx context.Context, orgID uuid.UUID, role string) (int, error) {
	return s.db.NewSelect().
		Model((*model.User)(nil)).
		Where("org_id = ? AND role = ?", orgID, role).
		Count(ctx)
}
