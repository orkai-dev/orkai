package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			// Pages: static sites deployed to cloud CDN + object storage (no K8s).
			// One Page -> one S3 origin -> one CloudFront distribution.
			`CREATE TABLE IF NOT EXISTS pages (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				description TEXT DEFAULT '',
				git_repo TEXT DEFAULT '',
				git_branch VARCHAR(255) DEFAULT 'main',
				git_provider_id UUID DEFAULT NULL REFERENCES shared_resources(id) ON DELETE SET NULL,
				publish_path VARCHAR(1024) DEFAULT '.',
				provider VARCHAR(50) DEFAULT 'aws_cloudfront',
				cloud_account_id UUID DEFAULT NULL REFERENCES shared_resources(id) ON DELETE SET NULL,
				region VARCHAR(50) DEFAULT 'us-east-1',
				runtime JSONB DEFAULT '{}',
				status VARCHAR(50) DEFAULT 'idle',
				-- webhook_secret and auto_deploy are created here to avoid an extra
				-- migration in Phase 2, but are UNWIRED until then. Do not assume
				-- they are functional.
				webhook_secret VARCHAR(255) DEFAULT '',
				auto_deploy BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				UNIQUE(project_id, name)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_pages_project_id ON pages(project_id)`,

			// Page deployments: a single clone + sync run. MVP has no "building"
			// status — only "deploying".
			`CREATE TABLE IF NOT EXISTS page_deployments (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
				status VARCHAR(50) DEFAULT 'queued',
				-- Snapshot of the publish folder synced by this deploy; part of the
				-- dedup key with commit_sha so a publish_path change without a new
				-- commit still re-syncs.
				publish_path VARCHAR(1024) DEFAULT '.',
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
			`CREATE INDEX IF NOT EXISTS idx_page_deployments_page_id ON page_deployments(page_id)`,
			// Supports the per-deploy dedup lookup (GetLatestSuccess:
			// WHERE page_id = ? AND status = 'success' ORDER BY created_at DESC
			// LIMIT 1) and GetLatestByPage, so they stay O(index seek) as
			// deployment history grows.
			`CREATE INDEX IF NOT EXISTS idx_page_deployments_page_status_created ON page_deployments(page_id, status, created_at DESC)`,

			// updated_at + real-time SSE triggers (same pattern as other tables).
			`CREATE OR REPLACE TRIGGER trigger_pages_updated_at
				BEFORE UPDATE ON pages
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
			`CREATE OR REPLACE TRIGGER trigger_pages_notify
				AFTER INSERT OR UPDATE OR DELETE ON pages
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
			`CREATE OR REPLACE TRIGGER trigger_page_deployments_updated_at
				BEFORE UPDATE ON page_deployments
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
			`CREATE OR REPLACE TRIGGER trigger_page_deployments_notify
				AFTER INSERT OR UPDATE OR DELETE ON page_deployments
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("pages migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`DROP TABLE IF EXISTS page_deployments CASCADE`,
			`DROP TABLE IF EXISTS pages CASCADE`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
