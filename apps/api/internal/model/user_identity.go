package model

import "github.com/google/uuid"

// UserIdentity links an external OAuth provider subject to a Orkai user.
type UserIdentity struct {
	BaseModel `bun:"table:user_identities,alias:ui"`

	UserID   uuid.UUID `bun:"user_id,notnull,type:uuid" json:"user_id"`
	Provider string    `bun:"provider,notnull" json:"provider"`
	Subject  string    `bun:"subject,notnull" json:"subject"`
	Email    string    `bun:"email,notnull,default:''" json:"email"`
}
