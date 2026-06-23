package pg

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type oauthChallengeStore struct {
	db bun.IDB
}

func (s *oauthChallengeStore) Consume(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	res, err := s.db.NewInsert().
		Model(&model.UsedOAuthChallenge{JTI: jti, ExpiresAt: expiresAt}).
		On("CONFLICT (jti) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	// Prune expired rows opportunistically to keep the table bounded. This is
	// pure housekeeping: its outcome must never affect the single-use decision
	// above, so any error here is intentionally ignored rather than returned.
	_, _ = s.db.NewDelete().
		Model((*model.UsedOAuthChallenge)(nil)).
		Where("expires_at < ?", time.Now()).
		Exec(ctx)

	return n > 0, nil
}
