package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
)

type sharedResourceStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *sharedResourceStore) encryptResource(resource *model.SharedResource) error {
	if resource == nil || len(resource.Config) == 0 {
		return nil
	}
	enc, err := encryptResourceConfig(s.secrets, resource.Config)
	if err != nil {
		return err
	}
	resource.Config = enc
	return nil
}

func (s *sharedResourceStore) decryptResource(resource *model.SharedResource) error {
	if resource == nil || len(resource.Config) == 0 {
		return nil
	}
	dec, err := decryptResourceConfig(s.secrets, resource.Config)
	if err != nil {
		return err
	}
	resource.Config = dec
	return nil
}

func (s *sharedResourceStore) GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
	resource := new(model.SharedResource)
	err := s.db.NewSelect().Model(resource).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.decryptResource(resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *sharedResourceStore) Create(ctx context.Context, resource *model.SharedResource) error {
	if err := s.encryptResource(resource); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(resource).Exec(ctx)
	if err != nil {
		return err
	}
	return s.decryptResource(resource)
}

func (s *sharedResourceStore) Update(ctx context.Context, resource *model.SharedResource) error {
	if err := s.encryptResource(resource); err != nil {
		return err
	}
	_, err := s.db.NewUpdate().Model(resource).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	return s.decryptResource(resource)
}

func (s *sharedResourceStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.SharedResource)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *sharedResourceStore) ListByOrg(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error) {
	var resources []model.SharedResource
	q := s.db.NewSelect().Model(&resources).Where("org_id = ?", orgID)
	if resourceType != "" {
		q = q.Where("type = ?", resourceType)
	}
	err := q.OrderExpr("created_at DESC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	for i := range resources {
		if err := s.decryptResource(&resources[i]); err != nil {
			return nil, err
		}
	}
	return resources, nil
}
