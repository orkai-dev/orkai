package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// likeEscaper escapes LIKE metacharacters so a user-supplied search term is
// matched literally instead of being interpreted as wildcards. Pair it with an
// `ESCAPE '\'` clause. The backslash is escaped first so it can't break the
// escape clause itself.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// likeContains builds a case-insensitive "contains" LIKE pattern (%term%) with
// metacharacters escaped. Use with: Where("LOWER(col) LIKE LOWER(?) ESCAPE '\\'", likeContains(term)).
func likeContains(term string) string {
	return "%" + likeEscaper.Replace(term) + "%"
}

// Store implements store.Store backed by PostgreSQL using Bun ORM.
type Store struct {
	db      *bun.DB // concrete handle for DB()/Close()/migrations
	idb     bun.IDB // executor: *bun.DB at top level, bun.Tx inside Tx
	secrets secret.Store
}

// PoolConfig holds connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// New creates a new PostgreSQL-backed Store with connection retry and pool config.
func New(databaseURL string, secrets secret.Store, pool ...PoolConfig) (*Store, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))

	// Apply pool settings (enables automatic reconnect on broken connections)
	if len(pool) > 0 {
		p := pool[0]
		if p.MaxOpenConns > 0 {
			sqldb.SetMaxOpenConns(p.MaxOpenConns)
		}
		if p.MaxIdleConns > 0 {
			sqldb.SetMaxIdleConns(p.MaxIdleConns)
		}
		if p.ConnMaxLifetime > 0 {
			sqldb.SetConnMaxLifetime(p.ConnMaxLifetime)
		}
	} else {
		sqldb.SetMaxOpenConns(25)
		sqldb.SetMaxIdleConns(5)
		sqldb.SetConnMaxLifetime(5 * time.Minute)
	}

	db := bun.NewDB(sqldb, pgdialect.New())

	// Retry connection — PG may still be starting
	var err error
	for i := range 30 {
		if err = db.Ping(); err == nil {
			return &Store{db: db, idb: db, secrets: secrets}, nil
		}
		if i < 29 {
			time.Sleep(time.Second)
		}
	}

	return nil, fmt.Errorf("database not reachable after 30s: %w", err)
}

// DB returns the underlying bun.DB for use in migrations.
func (s *Store) DB() *bun.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Tx(ctx context.Context, fn func(ctx context.Context, tx store.Store) error) error {
	return s.idb.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, &Store{db: s.db, idb: tx, secrets: s.secrets})
	})
}

func (s *Store) Organizations() store.OrganizationStore { return &organizationStore{db: s.idb} }
func (s *Store) Users() store.UserStore {
	return &userStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) Projects() store.ProjectStore { return &projectStore{db: s.idb} }
func (s *Store) Applications() store.ApplicationStore {
	return &applicationStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) Deployments() store.DeploymentStore { return &deploymentStore{db: s.idb} }
func (s *Store) Domains() store.DomainStore         { return &domainStore{db: s.idb} }
func (s *Store) ManagedDatabases() store.ManagedDatabaseStore {
	return &managedDatabaseStore{db: s.idb}
}
func (s *Store) Templates() store.TemplateStore { return &templateStore{db: s.idb} }
func (s *Store) Settings() store.SettingStore {
	return &settingStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) ServerNodes() store.ServerNodeStore {
	return &serverNodeStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) SharedResources() store.SharedResourceStore {
	return &sharedResourceStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) CronJobs() store.CronJobStore       { return &cronJobStore{db: s.idb} }
func (s *Store) CronJobRuns() store.CronJobRunStore { return &cronJobRunStore{db: s.idb} }
func (s *Store) DatabaseBackups() store.DatabaseBackupStore {
	return &databaseBackupStore{db: s.idb}
}
func (s *Store) Teams() store.TeamStore             { return &teamStore{db: s.idb} }
func (s *Store) TeamMembers() store.TeamMemberStore { return &teamMemberStore{db: s.idb} }
func (s *Store) Invitations() store.InvitationStore { return &invitationStore{db: s.idb} }
func (s *Store) NotificationChannels() store.NotificationChannelStore {
	return &notificationChannelStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) SystemBackups() store.SystemBackupStore { return &systemBackupStore{db: s.idb} }
func (s *Store) Identities() store.IdentityStore        { return &identityStore{db: s.idb} }
func (s *Store) OAuthChallenges() store.OAuthChallengeStore {
	return &oauthChallengeStore{db: s.idb}
}
func (s *Store) DeployTargets() store.DeployTargetStore { return &deployTargetStore{db: s.idb} }
func (s *Store) Pages() store.PageStore {
	return &pageStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) PageDeployments() store.PageDeploymentStore {
	return &pageDeploymentStore{db: s.idb}
}
func (s *Store) Workers() store.WorkerStore {
	return &workerStore{db: s.idb, secrets: s.secrets}
}
func (s *Store) WorkerDeployments() store.WorkerDeploymentStore {
	return &workerDeploymentStore{db: s.idb}
}
func (s *Store) APIKeys() store.APIKeyStore { return &apiKeyStore{db: s.idb} }
