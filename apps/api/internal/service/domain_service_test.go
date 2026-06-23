package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDomainService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *DomainService {
	ss := NewSettingService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger())
	return NewDomainService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger(), ss, nil)
}

func TestNormalizeDomainHost(t *testing.T) {
	assert.Equal(t, "example.com", normalizeDomainHost("  Example.com.  "))
}

func TestValidateDomainHost(t *testing.T) {
	require.NoError(t, validateDomainHost("app.example.com"))
	require.NoError(t, validateDomainHost("localhost"))
	require.Error(t, validateDomainHost(""))
	require.Error(t, validateDomainHost(string(make([]byte, 254))))
	require.Error(t, validateDomainHost("a..b"))
	require.Error(t, validateDomainHost("-bad.example.com"))
	require.Error(t, validateDomainHost("bad-.example.com"))
	require.Error(t, validateDomainHost("inv@lid.com"))
	require.Error(t, validateDomainHost("singlelabel"))
}

func TestRandomShort(t *testing.T) {
	assert.Len(t, randomShort(4), 4)
}

func TestDomainCreateInvalidHost(t *testing.T) {
	s := newDomainService(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateDomainInput{Host: "bad host"})
	require.Error(t, err)
}

func TestDomainCreateAppNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, errors.New("missing")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateDomainInput{Host: "app.example.com"})
	require.Error(t, err)
}

func TestDomainCreateDuplicate(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	fs.DomainsStore.CreateFn = func(ctx context.Context, d *model.Domain) error {
		return errors.New("duplicate key")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateDomainInput{Host: "app.example.com"})
	require.ErrorContains(t, err, "already in use")
}

func TestDomainCreateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	fs.DomainsStore.CreateFn = func(ctx context.Context, d *model.Domain) error {
		return errors.New("disk full")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateDomainInput{Host: "app.example.com"})
	require.Error(t, err)
}

func TestDomainCreateIngressFailRollback(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	deleted := false
	fs.DomainsStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.CreateIngressFn = func(ctx context.Context, d *model.Domain, app *model.Application) error {
		return errors.New("ingress failed")
	}
	s := newDomainService(fs, orch)
	_, err := s.Create(context.Background(), uuid.New(), CreateDomainInput{Host: "app.example.com"})
	require.ErrorContains(t, err, "create ingress failed")
	assert.True(t, deleted)
}

func TestDomainCreateSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	d, err := s.Create(context.Background(), uuid.New(), CreateDomainInput{Host: "app.example.com", TLS: true})
	require.NoError(t, err)
	assert.Equal(t, "app.example.com", d.Host)
}

func TestGenerateTraefikDomainNoBaseDomain(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.GenerateTraefikDomain(context.Background(), uuid.New())
	require.ErrorContains(t, err, "base domain not configured")
}

func TestGenerateTraefikDomainReuseExisting(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingBaseDomain {
			return "example.com", nil
		}
		return "", nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
		return []model.Domain{{Host: "web-abcd.example.com"}}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	d, err := s.GenerateTraefikDomain(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "web-abcd.example.com", d.Host)
}

func TestGenerateTraefikDomainDev(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingBaseDomain {
			return "10.0.0.1.sslip.io", nil
		}
		return "", nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "My App!!"}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	d, err := s.GenerateTraefikDomain(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, d.TLS, "dev domains should not use TLS")
	assert.Contains(t, d.Host, ".sslip.io")
}

func TestGenerateTraefikDomainEmptyNameProd(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingBaseDomain {
			return "example.com", nil
		}
		return "", nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "!!!"}, nil // sanitizes to empty → "app"
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	d, err := s.GenerateTraefikDomain(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, d.Host, "app-")
	assert.True(t, d.TLS)
}

func TestGenerateTraefikDomainIngressFailRollback(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingBaseDomain {
			return "example.com", nil
		}
		return "", nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	deleted := false
	fs.DomainsStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.CreateIngressFn = func(ctx context.Context, d *model.Domain, app *model.Application) error {
		return errors.New("ingress failed")
	}
	s := newDomainService(fs, orch)
	_, err := s.GenerateTraefikDomain(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, deleted)
}

func TestDomainUpdateGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return nil, errors.New("missing")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), nil, nil)
	require.Error(t, err)
}

func TestDomainUpdateAppNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, errors.New("missing")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), nil, nil)
	require.Error(t, err)
}

func TestDomainUpdateInvalidHost(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "old.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	bad := "bad host"
	_, err := s.Update(context.Background(), uuid.New(), &bad, nil)
	require.Error(t, err)
}

func TestDomainUpdateManualCertRenameBlocked(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "old.example.com", TLS: true, AutoCert: false}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	newHost := "new.example.com"
	_, err := s.Update(context.Background(), uuid.New(), &newHost, nil)
	require.ErrorContains(t, err, "manually configured TLS")
}

func TestDomainUpdateHostChangeSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{BaseModel: model.BaseModel{ID: uuid.New()}, Host: "old.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	newHost := "new.example.com"
	d, err := s.Update(context.Background(), uuid.New(), &newHost, nil)
	require.NoError(t, err)
	assert.Equal(t, "new.example.com", d.Host)
}

func TestDomainUpdateHostChangeDuplicate(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "old.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	fs.DomainsStore.UpdateFn = func(ctx context.Context, d *model.Domain) error {
		return errors.New("unique violation")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	newHost := "new.example.com"
	_, err := s.Update(context.Background(), uuid.New(), &newHost, nil)
	require.ErrorContains(t, err, "already in use")
}

func TestDomainUpdateHostChangeIngressFailRollback(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "old.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.CreateIngressFn = func(ctx context.Context, d *model.Domain, app *model.Application) error {
		return errors.New("ingress failed")
	}
	s := newDomainService(fs, orch)
	newHost := "new.example.com"
	_, err := s.Update(context.Background(), uuid.New(), &newHost, nil)
	require.ErrorContains(t, err, "failed to create ingress")
}

func TestDomainUpdateForceHTTPSOnly(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "old.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	force := true
	d, err := s.Update(context.Background(), uuid.New(), nil, &force)
	require.NoError(t, err)
	assert.True(t, d.ForceHTTPS)
}

func TestDomainUpdateNonHostIngressError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "old.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.UpdateIngressFn = func(ctx context.Context, d *model.Domain, app *model.Application) error {
		return errors.New("update ingress failed")
	}
	s := newDomainService(fs, orch)
	force := true
	_, err := s.Update(context.Background(), uuid.New(), nil, &force)
	require.ErrorContains(t, err, "ingress not updated")
}

func TestDomainListByApp(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
		return []model.Domain{{Host: "a.example.com", TLS: true, CertSecret: "old"}}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	domains, err := s.ListByApp(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, domains, 1)
}

func TestDomainListByAppStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
		return nil, errors.New("boom")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.ListByApp(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestDomainListByAppAppLookupFails(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
		return []model.Domain{{Host: "a.example.com"}}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, errors.New("missing")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	domains, err := s.ListByApp(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, domains, 1)
}

func TestDomainDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "a.example.com"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "api"}, nil
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Delete(context.Background(), uuid.New()))
}

func TestDomainDeleteGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return nil, errors.New("missing")
	}
	s := newDomainService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Delete(context.Background(), uuid.New()))
}

func TestDomainDeleteIngressError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DomainsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{Host: "a.example.com"}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeleteIngressFn = func(ctx context.Context, d *model.Domain) error {
		return errors.New("ingress delete failed")
	}
	s := newDomainService(fs, orch)
	require.ErrorContains(t, s.Delete(context.Background(), uuid.New()), "domain not deleted")
}
