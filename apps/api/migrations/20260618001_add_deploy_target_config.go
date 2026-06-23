package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE deploy_targets
			ADD COLUMN IF NOT EXISTS config JSONB NOT NULL DEFAULT '{}'
		`)
		if err != nil {
			return fmt.Errorf("add deploy target config: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE deploy_targets
			DROP COLUMN IF EXISTS config
		`)
		if err != nil {
			return fmt.Errorf("drop deploy target config: %w", err)
		}
		return nil
	})
}
