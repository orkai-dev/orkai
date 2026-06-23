package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE server_nodes ADD COLUMN IF NOT EXISTS host_key_fingerprint TEXT DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("add host_key_fingerprint: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE server_nodes DROP COLUMN IF EXISTS host_key_fingerprint`)
		return err
	})
}
