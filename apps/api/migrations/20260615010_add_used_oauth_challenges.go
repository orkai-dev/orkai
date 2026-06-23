package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS used_oauth_challenges (
    jti UUID PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
)`)
		if err != nil {
			return fmt.Errorf("create used_oauth_challenges: %w", err)
		}

		_, err = db.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_used_oauth_challenges_expires_at ON used_oauth_challenges(expires_at)`)
		if err != nil {
			return fmt.Errorf("create used_oauth_challenges index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS used_oauth_challenges`)
		return err
	})
}
