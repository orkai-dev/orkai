package pg

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type identityStore struct {
	db bun.IDB
}

func (s *identityStore) GetByProviderSubject(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
	identity := new(model.UserIdentity)
	err := s.db.NewSelect().
		Model(identity).
		Where("provider = ? AND subject = ?", provider, subject).
		Scan(ctx)
	return identity, err
}

func (s *identityStore) Create(ctx context.Context, identity *model.UserIdentity) error {
	_, err := s.db.NewInsert().Model(identity).Returning("*").Exec(ctx)
	return err
}
