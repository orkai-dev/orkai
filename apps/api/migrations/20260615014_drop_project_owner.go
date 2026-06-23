package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects DROP COLUMN IF EXISTS owner`); err != nil {
			return fmt.Errorf("drop project owner: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `ALTER TABLE projects ADD COLUMN IF NOT EXISTS owner VARCHAR(255) DEFAULT ''`); err != nil {
			return fmt.Errorf("restore project owner: %w", err)
		}
		return nil
	})
}
