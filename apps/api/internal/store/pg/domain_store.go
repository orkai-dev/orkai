package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type domainStore struct {
	db bun.IDB
}

func (s *domainStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
	domain := new(model.Domain)
	err := s.db.NewSelect().Model(domain).Where("id = ?", id).Scan(ctx)
	return domain, err
}

func (s *domainStore) Create(ctx context.Context, domain *model.Domain) error {
	_, err := s.db.NewInsert().Model(domain).Returning("*").Exec(ctx)
	return err
}

func (s *domainStore) Update(ctx context.Context, domain *model.Domain) error {
	_, err := s.db.NewUpdate().Model(domain).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *domainStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Domain)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *domainStore) ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
	var domains []model.Domain
	err := s.db.NewSelect().Model(&domains).Where("app_id = ?", appID).Scan(ctx)
	return domains, err
}

func (s *domainStore) GetByHost(ctx context.Context, host string) (*model.Domain, error) {
	domain := new(model.Domain)
	err := s.db.NewSelect().Model(domain).Where("host = ?", host).Scan(ctx)
	return domain, err
}

// ListAllHosts returns every domain host in a single query. It is intentionally
// unbounded: orphan-ingress detection compares live cluster ingresses against
// this set, so a truncated list would misclassify valid ingresses as orphans and
// risk deleting them. Hosts are unique and short, so the full set is cheap.
func (s *domainStore) ListAllHosts(ctx context.Context) ([]string, error) {
	hosts := []string{}
	err := s.db.NewSelect().
		Model((*model.Domain)(nil)).
		Column("host").
		Scan(ctx, &hosts)
	if err != nil {
		return nil, err
	}
	return hosts, nil
}
