package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS custom_domain VARCHAR(255) DEFAULT ''`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS manage_dns BOOLEAN DEFAULT false`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS dns_account_id UUID DEFAULT NULL REFERENCES shared_resources(id) ON DELETE SET NULL`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS dns_zone_id VARCHAR(255) DEFAULT ''`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("pages custom domain migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE pages DROP COLUMN IF EXISTS dns_zone_id`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS dns_account_id`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS manage_dns`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS custom_domain`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
