package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// UserIdentity embeds BaseModel, which declares a soft_delete deleted_at
		// field. Bun therefore scopes every query with `deleted_at IS NULL`, but
		// the original create-table migration omitted the column, so OAuth login
		// failed with `column "deleted_at" ... does not exist` (SQLSTATE 42703).
		_, err := db.ExecContext(ctx,
			`ALTER TABLE user_identities ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`)
		if err != nil {
			return fmt.Errorf("add user_identities.deleted_at: %w", err)
		}

		// Replace the table-level UNIQUE (provider, subject) with a partial unique
		// index over active rows only, so a soft-deleted identity doesn't reserve
		// the (provider, subject) pair forever (matches users/domains/pages).
		_, _ = db.ExecContext(ctx,
			`ALTER TABLE user_identities DROP CONSTRAINT IF EXISTS user_identities_provider_subject_key`)
		_, err = db.ExecContext(ctx,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_identities_provider_subject_active
			 ON user_identities(provider, subject) WHERE deleted_at IS NULL`)
		if err != nil {
			return fmt.Errorf("create user_identities partial unique index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Restore the table-level constraint before dropping the partial index so a
		// failed rollback leaves the active-only index in place.
		_, err := db.ExecContext(ctx,
			`ALTER TABLE user_identities ADD CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject)`)
		if err != nil {
			return fmt.Errorf("restore user_identities provider/subject unique constraint: %w", err)
		}
		_, _ = db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_user_identities_provider_subject_active`)
		_, err = db.ExecContext(ctx, `ALTER TABLE user_identities DROP COLUMN IF EXISTS deleted_at`)
		if err != nil {
			return fmt.Errorf("drop user_identities.deleted_at: %w", err)
		}
		return nil
	})
}
