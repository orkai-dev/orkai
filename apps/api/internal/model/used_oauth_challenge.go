package model

import (
	"time"

	"github.com/uptrace/bun"
)

// UsedOAuthChallenge records an OAuth 2FA challenge token that has already been
// redeemed, enforcing single use across API instances and restarts. Rows are
// pruned once expired.
type UsedOAuthChallenge struct {
	bun.BaseModel `bun:"table:used_oauth_challenges,alias:uoc"`

	JTI       string    `bun:"jti,pk" json:"jti"`
	ExpiresAt time.Time `bun:"expires_at,notnull" json:"expires_at"`
}
