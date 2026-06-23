package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			// Workers: Cloudflare Workers deployed from a git repo via
			// `wrangler deploy` in an in-cluster build pod (no K8s workload).
			`CREATE TABLE IF NOT EXISTS workers (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				description TEXT DEFAULT '',
				git_repo TEXT DEFAULT '',
				git_branch VARCHAR(255) DEFAULT 'main',
				git_provider_id UUID DEFAULT NULL REFERENCES shared_resources(id) ON DELETE SET NULL,
				root_directory VARCHAR(1024) DEFAULT '.',
				wrangler_config VARCHAR(1024) DEFAULT 'wrangler.toml',
				package_manager VARCHAR(50) DEFAULT 'auto',
				install_command TEXT DEFAULT '',
				build_env_vars JSONB DEFAULT '{}',
				cloud_account_id UUID DEFAULT NULL REFERENCES shared_resources(id) ON DELETE SET NULL,
				runtime JSONB DEFAULT '{}',
				status VARCHAR(50) DEFAULT 'idle',
				webhook_secret VARCHAR(255) DEFAULT '',
				auto_deploy BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_workers_project_id ON workers(project_id)`,
			`CREATE INDEX IF NOT EXISTS idx_workers_cloud_account_id ON workers(cloud_account_id)`,
			`CREATE INDEX IF NOT EXISTS idx_workers_git_provider_id ON workers(git_provider_id)`,
			// Soft-delete aware uniqueness: reuse a name after deleting a worker.
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_workers_project_id_name_active ON workers(project_id, name) WHERE deleted_at IS NULL`,

			// Worker deployments: a single clone + install + wrangler deploy run.
			`CREATE TABLE IF NOT EXISTS worker_deployments (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				worker_id UUID NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
				status VARCHAR(50) DEFAULT 'queued',
				commit_sha VARCHAR(255) DEFAULT '',
				deploy_log TEXT DEFAULT '',
				provider_ref VARCHAR(255) DEFAULT '',
				trigger_type VARCHAR(50) DEFAULT 'manual',
				started_at TIMESTAMPTZ,
				finished_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_worker_deployments_worker_id ON worker_deployments(worker_id)`,
			// Supports GetLatestByWorker / GetLatestSuccess + stale recovery so
			// they stay O(index seek) as deployment history grows.
			`CREATE INDEX IF NOT EXISTS idx_worker_deployments_worker_status_created ON worker_deployments(worker_id, status, created_at DESC)`,

			// updated_at + real-time SSE triggers (same pattern as other tables).
			`CREATE OR REPLACE TRIGGER trigger_workers_updated_at
				BEFORE UPDATE ON workers
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
			`CREATE OR REPLACE TRIGGER trigger_workers_notify
				AFTER INSERT OR UPDATE OR DELETE ON workers
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
			`CREATE OR REPLACE TRIGGER trigger_worker_deployments_updated_at
				BEFORE UPDATE ON worker_deployments
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
			`CREATE OR REPLACE TRIGGER trigger_worker_deployments_notify
				AFTER INSERT OR UPDATE OR DELETE ON worker_deployments
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("workers migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`DROP TABLE IF EXISTS worker_deployments CASCADE`,
			`DROP TABLE IF EXISTS workers CASCADE`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
