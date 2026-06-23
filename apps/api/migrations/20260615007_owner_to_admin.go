package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// The "owner" role was collapsed into "admin". Promote any legacy
		// owner rows so existing data stays valid under the new model.
		if _, err := db.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE role = 'owner'`); err != nil {
			return fmt.Errorf("owner to admin: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Irreversible: we cannot tell which admins were previously owners.
		return nil
	})
}
