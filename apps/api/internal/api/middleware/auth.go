package middleware

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// RateLimit returns middleware that limits requests per IP.
// maxAttempts per window duration.
func RateLimit(maxAttempts int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	attempts := make(map[string]*entry)

	// Cleanup old entries periodically
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for k, e := range attempts {
				if now.After(e.reset) {
					delete(attempts, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		e, ok := attempts[ip]
		if !ok || time.Now().After(e.reset) {
			attempts[ip] = &entry{count: 1, reset: time.Now().Add(window)}
			mu.Unlock()
			c.Next()
			return
		}
		e.count++
		if e.count > maxAttempts {
			mu.Unlock()
			httputil.RespondError(c, apierr.ErrTooManyRequests.WithDetail("too many attempts, try again later"))
			c.Abort()
			return
		}
		mu.Unlock()
		c.Next()
	}
}

const (
	// Context keys for user info
	CtxUserID = "user_id"
	CtxOrgID  = "org_id"
	CtxRole   = "role"

	sessionVersionCacheTTL = 30 * time.Second
)

type versionCacheEntry struct {
	version int
	expires time.Time
}

// SessionInvalidator clears cached token_version entries after revocation events.
type SessionInvalidator interface {
	Invalidate(userID uuid.UUID)
}

// APIKeyAuthenticator validates API keys for request authentication.
type APIKeyAuthenticator interface {
	GetByHash(ctx context.Context, hash string) (*model.APIKey, error)
	TouchLastUsed(ctx context.Context, id uuid.UUID) error
}

// SessionValidator validates JWT access tokens and checks token_version against the database.
type SessionValidator struct {
	jwtManager *auth.JWTManager
	users      store.UserStore
	apiKeys    APIKeyAuthenticator

	mu            sync.Mutex
	cache         map[uuid.UUID]versionCacheEntry
	invalidations map[uuid.UUID]uint64

	lastUsedMu sync.Mutex
	lastUsed   map[uuid.UUID]time.Time
}

// NewSessionValidator creates a validator that caches token_version lookups.
func NewSessionValidator(jwtManager *auth.JWTManager, users store.UserStore, apiKeys APIKeyAuthenticator) *SessionValidator {
	return &SessionValidator{
		jwtManager:    jwtManager,
		users:         users,
		apiKeys:       apiKeys,
		cache:         make(map[uuid.UUID]versionCacheEntry),
		invalidations: make(map[uuid.UUID]uint64),
		lastUsed:      make(map[uuid.UUID]time.Time),
	}
}

func (v *SessionValidator) currentVersion(ctx context.Context, userID uuid.UUID) (int, error) {
	for {
		now := time.Now()

		v.mu.Lock()
		if entry, ok := v.cache[userID]; ok && now.Before(entry.expires) {
			version := entry.version
			v.mu.Unlock()
			return version, nil
		}
		gen := v.invalidations[userID]
		v.mu.Unlock()

		user, err := v.users.GetByID(ctx, userID)
		if err != nil {
			return 0, err
		}

		v.mu.Lock()
		if v.invalidations[userID] != gen {
			v.mu.Unlock()
			continue // Invalidate ran during fetch; retry without caching stale data
		}
		v.cache[userID] = versionCacheEntry{
			version: user.TokenVersion,
			expires: now.Add(sessionVersionCacheTTL),
		}
		v.mu.Unlock()

		return user.TokenVersion, nil
	}
}

// Invalidate drops the cached token_version for a user so the next request
// loads the current value from the database. Call after bumping token_version.
func (v *SessionValidator) Invalidate(userID uuid.UUID) {
	v.mu.Lock()
	delete(v.cache, userID)
	v.invalidations[userID]++
	v.mu.Unlock()
}

func (v *SessionValidator) validateClaims(c *gin.Context, claims *auth.Claims) bool {
	current, err := v.currentVersion(c.Request.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("session invalidated"))
			return false
		}
		c.AbortWithStatusJSON(500, apierr.ErrInternal.WithDetail("authentication check failed"))
		return false
	}
	if claims.TokenVersion != current {
		c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("session invalidated"))
		return false
	}
	return true
}

// Auth returns a middleware that validates JWT tokens and token_version.
func (v *SessionValidator) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("missing authorization header"))
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("invalid authorization format"))
			return
		}

		token := parts[1]
		if strings.HasPrefix(token, auth.APIKeyPrefix) {
			if !v.authenticateAPIKey(c, token) {
				return
			}
			c.Next()
			return
		}

		claims, err := v.jwtManager.ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail(err.Error()))
			return
		}

		if !v.validateClaims(c, claims) {
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxOrgID, claims.OrgID)
		c.Set(CtxRole, claims.Role)

		c.Next()
	}
}

func (v *SessionValidator) authenticateAPIKey(c *gin.Context, token string) bool {
	if v.apiKeys == nil {
		c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("invalid api key"))
		return false
	}

	key, err := v.apiKeys.GetByHash(c.Request.Context(), auth.HashAPIKey(token))
	if err != nil {
		if pd, ok := err.(*apierr.ProblemDetail); ok {
			c.AbortWithStatusJSON(pd.Status, pd)
			return false
		}
		c.AbortWithStatusJSON(500, apierr.ErrInternal.WithDetail("authentication check failed"))
		return false
	}

	c.Set(CtxUserID, key.UserID)
	c.Set(CtxOrgID, key.OrgID)
	c.Set(CtxRole, string(key.Role))
	v.touchLastUsedAsync(c.Request.Context(), key.ID)
	return true
}

const apiKeyLastUsedInterval = 5 * time.Minute

func (v *SessionValidator) touchLastUsedAsync(ctx context.Context, keyID uuid.UUID) {
	now := time.Now()

	v.lastUsedMu.Lock()
	if last, ok := v.lastUsed[keyID]; ok && now.Sub(last) < apiKeyLastUsedInterval {
		v.lastUsedMu.Unlock()
		return
	}
	v.lastUsed[keyID] = now
	for id, last := range v.lastUsed {
		if id != keyID && now.Sub(last) >= apiKeyLastUsedInterval {
			delete(v.lastUsed, id)
		}
	}
	v.lastUsedMu.Unlock()

	go func() {
		if err := v.apiKeys.TouchLastUsed(context.WithoutCancel(ctx), keyID); err != nil {
			slog.Debug("api key last_used update failed", "key_id", keyID, "err", err)
		}
	}()
}

// WSAuth validates a JWT token from the query parameter "token".
// Used for WebSocket/SSE routes where Authorization headers can't be set.
func (v *SessionValidator) WSAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			// Fallback: try Authorization header (for SSE clients that support it)
			header := c.GetHeader("Authorization")
			if header != "" {
				parts := strings.SplitN(header, " ", 2)
				if len(parts) == 2 {
					token = parts[1]
				}
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "authentication required"})
			return
		}
		claims, err := v.jwtManager.ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}
		if !v.validateClaims(c, claims) {
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxOrgID, claims.OrgID)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// GetUserID extracts user ID from context.
func GetUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(CtxUserID); ok {
		return v.(uuid.UUID)
	}
	return uuid.Nil
}

// GetOrgID extracts organization ID from context.
func GetOrgID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(CtxOrgID); ok {
		return v.(uuid.UUID)
	}
	return uuid.Nil
}

// GetUserRole extracts user role from context.
func GetUserRole(c *gin.Context) string {
	if v, ok := c.Get(CtxRole); ok {
		return v.(string)
	}
	return ""
}

// RequireRole returns a middleware that restricts access to users with one of the given roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetUserRole(c)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, map[string]any{
			"type":   "https://orkai.dev/errors/forbidden",
			"title":  "Forbidden",
			"status": 403,
			"detail": "insufficient permissions",
		})
	}
}

// RequireSetupSecret validates the X-Setup-Secret header against the configured secret.
// Used to protect unauthenticated setup-only endpoints (e.g. system restore).
func RequireSetupSecret(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader("X-Setup-Secret")
		if provided == "" {
			httputil.RespondError(c, apierr.ErrUnauthorized.WithDetail("setup secret required"))
			c.Abort()
			return
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			httputil.RespondError(c, apierr.ErrForbidden.WithDetail("invalid setup secret"))
			c.Abort()
			return
		}
		c.Next()
	}
}
