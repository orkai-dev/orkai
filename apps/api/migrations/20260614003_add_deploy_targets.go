package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS deploy_targets (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
				kind VARCHAR(50) NOT NULL,
				region VARCHAR(100) DEFAULT '',
				capabilities JSONB NOT NULL DEFAULT '[]',
				is_default BOOLEAN NOT NULL DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_deploy_targets_default
			 ON deploy_targets (is_default) WHERE is_default = true AND deleted_at IS NULL`,
			`INSERT INTO deploy_targets (id, kind, region, capabilities, is_default)
			 VALUES (
				'00000000-0000-4000-8000-000000000001',
				'k3s',
				'',
				'["deploy","exec","volumes","managed_db","build","ingress","secrets","cron","logs","kubernetes"]'::jsonb,
				true
			 )
			 ON CONFLICT (id) DO NOTHING`,
			`ALTER TABLE applications ADD COLUMN IF NOT EXISTS target_id UUID REFERENCES deploy_targets(id)`,
			`ALTER TABLE applications ADD COLUMN IF NOT EXISTS provider VARCHAR(50) DEFAULT 'k3s'`,
			`ALTER TABLE applications ADD COLUMN IF NOT EXISTS region VARCHAR(100) DEFAULT ''`,
			`UPDATE applications SET provider = 'k3s' WHERE provider IS NULL OR provider = ''`,
			`UPDATE applications SET target_id = '00000000-0000-4000-8000-000000000001'
			 WHERE target_id IS NULL`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("add deploy targets: %w", err)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE applications DROP COLUMN IF EXISTS target_id`,
			`ALTER TABLE applications DROP COLUMN IF EXISTS provider`,
			`ALTER TABLE applications DROP COLUMN IF EXISTS region`,
			`DROP TABLE IF EXISTS deploy_targets`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
