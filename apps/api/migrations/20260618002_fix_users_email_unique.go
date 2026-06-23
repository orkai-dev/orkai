package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// users uses BaseModel soft delete (deleted_at). Removing a member
		// soft-deletes the row, but the table-level UNIQUE on email kept the
		// address reserved, blocking re-invites. Match the domains/pages fix:
		// partial unique index over active rows only.
		_, _ = db.ExecContext(ctx, `ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key`)
		_, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active ON users(email) WHERE deleted_at IS NULL`)
		if err != nil {
			return fmt.Errorf("create users partial unique index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Restore the table-level constraint before dropping the partial index so
		// a failed rollback (e.g. duplicate emails across soft-deleted rows)
		// leaves idx_users_email_active in place.
		_, err := db.ExecContext(ctx, `ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)`)
		if err != nil {
			return fmt.Errorf("restore users email unique constraint: %w", err)
		}
		_, _ = db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_users_email_active`)
		return nil
	})
}
