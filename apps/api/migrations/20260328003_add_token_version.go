package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version INTEGER DEFAULT 0`)
		if err != nil {
			return fmt.Errorf("add token_version: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE users DROP COLUMN IF EXISTS token_version`)
		return err
	})
}
