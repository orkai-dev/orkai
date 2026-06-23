package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS api_keys (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				key_prefix VARCHAR(32) NOT NULL,
				key_hash VARCHAR(64) NOT NULL,
				role VARCHAR(32) NOT NULL,
				last_used_at TIMESTAMPTZ,
				expires_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash)`,
			`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
			`CREATE OR REPLACE TRIGGER trigger_api_keys_updated_at
				BEFORE UPDATE ON api_keys
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
			`CREATE OR REPLACE TRIGGER trigger_api_keys_notify
				AFTER INSERT OR UPDATE OR DELETE ON api_keys
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("api_keys migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS api_keys CASCADE`)
		return err
	})
}
