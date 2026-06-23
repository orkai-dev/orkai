package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Drop the old unique constraint that doesn't respect soft deletes
		_, _ = db.ExecContext(ctx, `ALTER TABLE domains DROP CONSTRAINT IF EXISTS domains_host_key`)
		// Create a partial unique index that only applies to non-deleted rows
		_, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_host_active ON domains(host) WHERE deleted_at IS NULL`)
		if err != nil {
			return fmt.Errorf("create partial unique index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, _ = db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_domains_host_active`)
		_, err := db.ExecContext(ctx, `ALTER TABLE domains ADD CONSTRAINT domains_host_key UNIQUE (host)`)
		return err
	})
}
