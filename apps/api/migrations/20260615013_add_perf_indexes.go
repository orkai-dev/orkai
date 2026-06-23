package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			// FK columns hit by per-resource delete-guard and target lookups.
			// Without these every reference check is a sequential scan.
			`CREATE INDEX IF NOT EXISTS idx_applications_target_id ON applications(target_id)`,
			`CREATE INDEX IF NOT EXISTS idx_applications_registry_id ON applications(registry_id) WHERE registry_id IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_applications_git_provider_id ON applications(git_provider_id) WHERE git_provider_id IS NOT NULL`,
			// Backs the per-project K8s name-collision check on app/database create.
			`CREATE INDEX IF NOT EXISTS idx_applications_project_k8s_name ON applications(project_id, k8s_name)`,
			`CREATE INDEX IF NOT EXISTS idx_managed_databases_project_k8s_name ON managed_databases(project_id, k8s_name)`,
			`CREATE INDEX IF NOT EXISTS idx_managed_databases_backup_s3_id ON managed_databases(backup_s3_id) WHERE backup_s3_id IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_server_nodes_ssh_key_id ON server_nodes(ssh_key_id) WHERE ssh_key_id IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_pages_cloud_account_id ON pages(cloud_account_id) WHERE cloud_account_id IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_pages_git_provider_id ON pages(git_provider_id) WHERE git_provider_id IS NOT NULL`,
			// Global deployment list filters by status and orders by created_at;
			// mirror the composite already present on page_deployments.
			`CREATE INDEX IF NOT EXISTS idx_deployments_status_created ON deployments(status, created_at DESC)`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("add perf indexes: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`DROP INDEX IF EXISTS idx_applications_target_id`,
			`DROP INDEX IF EXISTS idx_applications_registry_id`,
			`DROP INDEX IF EXISTS idx_applications_git_provider_id`,
			`DROP INDEX IF EXISTS idx_applications_project_k8s_name`,
			`DROP INDEX IF EXISTS idx_managed_databases_project_k8s_name`,
			`DROP INDEX IF EXISTS idx_managed_databases_backup_s3_id`,
			`DROP INDEX IF EXISTS idx_server_nodes_ssh_key_id`,
			`DROP INDEX IF EXISTS idx_pages_cloud_account_id`,
			`DROP INDEX IF EXISTS idx_pages_git_provider_id`,
			`DROP INDEX IF EXISTS idx_deployments_status_created`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
