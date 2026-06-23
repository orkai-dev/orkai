package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE deployments
			ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN NOT NULL DEFAULT false
		`)
		if err != nil {
			return fmt.Errorf("add deployments.cancel_requested: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE deployments DROP COLUMN IF EXISTS cancel_requested
		`)
		return err
	})
}
