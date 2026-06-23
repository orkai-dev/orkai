package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Team is a group of users within an organization. Projects belong to a team,
// and team membership grants access to all of that team's projects.
type Team struct {
	BaseModel `bun:"table:teams,alias:t"`

	OrgID       uuid.UUID `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Name        string    `bun:"name,notnull" json:"name"`
	Description string    `bun:"description,default:''" json:"description"`
}

// TeamMember is the join between a user and a team (many-to-many).
type TeamMember struct {
	bun.BaseModel `bun:"table:team_members,alias:tm"`

	ID        uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	TeamID    uuid.UUID `bun:"team_id,notnull,type:uuid" json:"team_id"`
	UserID    uuid.UUID `bun:"user_id,notnull,type:uuid" json:"user_id"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// Invitation represents a pending invitation to join an organization.
type Invitation struct {
	bun.BaseModel `bun:"table:invitations,alias:inv"`

	ID         uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	OrgID      uuid.UUID  `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Email      string     `bun:"email,notnull" json:"email"`
	Role       string     `bun:"role,notnull,default:'member'" json:"role"` // "admin" or "member"
	Token      string     `bun:"token,notnull" json:"-"`                    // never expose in JSON
	InvitedBy  *uuid.UUID `bun:"invited_by,type:uuid" json:"invited_by,omitempty"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	AcceptedAt *time.Time `bun:"accepted_at" json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// OrgMember is a view struct for the organization member list (user + role).
type OrgMember struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	AvatarURL   string    `json:"avatar_url"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}
