//go:build integration

package pg_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun/migrate"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/store/pg"
	"github.com/orkai-dev/orkai/apps/api/migrations"
)

// dockerAvailable reports whether a Docker daemon is reachable. Integration
// tests are skipped when Docker is unavailable so they don't break CI/local
// runs without a container runtime.
func dockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

const integrationSetupSecret = "01234567890123456789012345678901"

// newTestStore spins up a throwaway Postgres container, runs all migrations and
// returns a ready store plus a cleanup func.
func newTestStore(t *testing.T) (*pg.Store, func()) {
	t.Helper()
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("orkai"),
		tcpostgres.WithUsername("orkai"),
		tcpostgres.WithPassword("orkai"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	secrets := secret.NewFromSetupSecret(integrationSetupSecret)
	st, err := pg.New(dsn, secrets)
	require.NoError(t, err)

	migrator := migrate.NewMigrator(st.DB(), migrations.Migrations)
	require.NoError(t, migrator.Init(ctx))
	_, err = migrator.Migrate(ctx)
	require.NoError(t, err)

	cleanup := func() {
		_ = st.Close()
		_ = container.Terminate(ctx)
	}
	return st, cleanup
}

func TestIntegrationSettingsCRUD(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// Missing key returns empty string, no error.
	v, err := st.Settings().Get(ctx, "missing")
	require.NoError(t, err)
	assert.Empty(t, v)

	// Set then read back.
	require.NoError(t, st.Settings().Set(ctx, "panel_domain", "example.com"))
	v, err = st.Settings().Get(ctx, "panel_domain")
	require.NoError(t, err)
	assert.Equal(t, "example.com", v)

	// Upsert overwrites.
	require.NoError(t, st.Settings().Set(ctx, "panel_domain", "new.example.com"))
	v, _ = st.Settings().Get(ctx, "panel_domain")
	assert.Equal(t, "new.example.com", v)

	all, err := st.Settings().GetAll(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, all)
}

func TestIntegrationOrgUserProject(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	org := &model.Organization{Name: "Acme"}
	require.NoError(t, st.Organizations().Create(ctx, org))
	assert.NotEqual(t, uuid.Nil, org.ID)

	got, err := st.Organizations().GetByID(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, "Acme", got.Name)

	user := &model.User{
		OrgID:        org.ID,
		Email:        "a@example.com",
		PasswordHash: "x",
		DisplayName:  "Alice",
		Role:         model.RoleAdmin,
	}
	require.NoError(t, st.Users().Create(ctx, user))
	assert.NotEqual(t, uuid.Nil, user.ID)

	byEmail, err := st.Users().GetByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, byEmail.ID)

	count, err := st.Users().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	team := &model.Team{OrgID: org.ID, Name: "core"}
	require.NoError(t, st.Teams().Create(ctx, team))

	project := &model.Project{
		OrgID:       org.ID,
		TeamID:      team.ID,
		Name:        "web",
		Environment: model.EnvProd,
	}
	require.NoError(t, st.Projects().Create(ctx, project))
	assert.NotEqual(t, uuid.Nil, project.ID)

	gotProject, err := st.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, "web", gotProject.Name)
	assert.NotNil(t, gotProject.EnvVars) // AfterScanRow initialises nil maps
	require.NotNil(t, gotProject.Team)
	assert.Equal(t, "core", gotProject.Team.Name)

	projects, total, err := st.Projects().ListByOrg(ctx, org.ID, store.ListParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, projects, 1)
	require.NotNil(t, projects[0].Team)
	assert.Equal(t, "core", projects[0].Team.Name)

	byTeam, total, err := st.Projects().ListByTeams(ctx, org.ID, []uuid.UUID{team.ID}, store.ListParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, byTeam, 1)
	require.NotNil(t, byTeam[0].Team)
	assert.Equal(t, "core", byTeam[0].Team.Name)
}
