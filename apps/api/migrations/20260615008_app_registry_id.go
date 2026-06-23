package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE applications ADD COLUMN IF NOT EXISTS registry_id UUID`)
		if err != nil {
			return fmt.Errorf("add registry_id: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE applications DROP COLUMN IF EXISTS registry_id`)
		return err
	})
}
