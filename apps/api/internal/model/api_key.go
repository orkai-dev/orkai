package model

import (
	"time"

	"github.com/google/uuid"
)

// APIKey is a personal access token for REST API authentication.
type APIKey struct {
	BaseModel `bun:"table:api_keys,alias:ak"`

	OrgID  uuid.UUID `bun:"org_id,notnull,type:uuid" json:"org_id"`
	UserID uuid.UUID `bun:"user_id,notnull,type:uuid" json:"user_id"`

	Name      string `bun:"name,notnull" json:"name"`
	KeyPrefix string `bun:"key_prefix,notnull" json:"key_prefix"`
	KeyHash   string `bun:"key_hash,notnull" json:"-"`
	Role      Role   `bun:"role,notnull" json:"role"`

	LastUsedAt *time.Time `bun:"last_used_at" json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
}
