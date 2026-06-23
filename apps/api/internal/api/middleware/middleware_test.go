package middleware

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

func TestRedactQuery(t *testing.T) {
	// Sensitive params are replaced.
	assert.Equal(t, "token=REDACTED", redactQuery("token=eyJhbGciOi.secret.jwt"))
	assert.Contains(t, redactQuery("token=abc&page=2"), "token=REDACTED")
	assert.Contains(t, redactQuery("token=abc&page=2"), "page=2")
	// Non-sensitive query is returned unchanged.
	assert.Equal(t, "page=2&size=10", redactQuery("page=2&size=10"))
	// access_token / refresh_token are also redacted.
	assert.Equal(t, "access_token=REDACTED", redactQuery("access_token=xyz"))
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newRouter() *gin.Engine {
	r := gin.New()
	return r
}

func doRequest(r *gin.Engine, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newJWT() *auth.JWTManager {
	return auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
}

type mockUserStore struct {
	users     map[uuid.UUID]*model.User
	getByIDFn func(ctx context.Context, id uuid.UUID) (*model.User, error)
}

func (m *mockUserStore) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserStore) GetByEmail(context.Context, string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserStore) Create(context.Context, *model.User) error { return nil }

func (m *mockUserStore) Update(context.Context, *model.User) error { return nil }

func (m *mockUserStore) UpdatePassword(context.Context, uuid.UUID, string) error { return nil }

func (m *mockUserStore) Update2FA(context.Context, uuid.UUID, bool, string) error { return nil }

func (m *mockUserStore) ListByOrg(context.Context, uuid.UUID, store.ListParams) ([]model.User, int, error) {
	return nil, 0, nil
}

func (m *mockUserStore) UpdateRole(context.Context, uuid.UUID, string) error { return nil }

func (m *mockUserStore) RemoveFromOrg(context.Context, uuid.UUID) error { return nil }

func (m *mockUserStore) BumpTokenVersion(context.Context, uuid.UUID) error { return nil }

func (m *mockUserStore) Count(context.Context) (int, error) { return 0, nil }

func (m *mockUserStore) CountByRole(context.Context, uuid.UUID, string) (int, error) {
	return 0, nil
}

func newSessionValidator(tokenVersion int) (*auth.JWTManager, *SessionValidator, *mockUserStore, uuid.UUID, uuid.UUID) {
	jm := newJWT()
	uid, oid := uuid.New(), uuid.New()
	users := &mockUserStore{
		users: map[uuid.UUID]*model.User{
			uid: {BaseModel: model.BaseModel{ID: uid}, TokenVersion: tokenVersion},
		},
	}
	return jm, NewSessionValidator(jm, users, nil), users, uid, oid
}

// ─── Auth ────────────────────────────────────────────────────────

func TestAuthMissingHeader(t *testing.T) {
	_, sv, _, _, _ := newSessionValidator(0)
	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", nil)
	assert.Equal(t, 401, w.Code)
}

func TestAuthBadFormat(t *testing.T) {
	_, sv, _, _, _ := newSessionValidator(0)
	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Token abc"})
	assert.Equal(t, 401, w.Code)
}

func TestAuthInvalidToken(t *testing.T) {
	_, sv, _, _, _ := newSessionValidator(0)
	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer not.a.jwt"})
	assert.Equal(t, 401, w.Code)
}

type mockAPIKeyAuth struct {
	getByHash func(ctx context.Context, hash string) (*model.APIKey, error)
}

func (m *mockAPIKeyAuth) GetByHash(ctx context.Context, hash string) (*model.APIKey, error) {
	return m.getByHash(ctx, hash)
}

func (m *mockAPIKeyAuth) TouchLastUsed(context.Context, uuid.UUID) error { return nil }

func apiKeyAuthFromStore(keys map[string]*model.APIKey) APIKeyAuthenticator {
	return &mockAPIKeyAuth{
		getByHash: func(_ context.Context, hash string) (*model.APIKey, error) {
			key, ok := keys[hash]
			if !ok {
				return nil, apierr.ErrUnauthorized.WithDetail("invalid api key")
			}
			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				return nil, apierr.ErrUnauthorized.WithDetail("api key expired")
			}
			return key, nil
		},
	}
}

func TestAuthValidAPIKey(t *testing.T) {
	jm, _, users, uid, oid := newSessionValidator(0)
	raw, _, hash, err := auth.GenerateAPIKey()
	require.NoError(t, err)

	sv := NewSessionValidator(jm, users, apiKeyAuthFromStore(map[string]*model.APIKey{
		hash: {
			BaseModel: model.BaseModel{ID: uuid.New()},
			UserID:    uid,
			OrgID:     oid,
			Role:      model.RoleAdmin,
		},
	}))

	r := newRouter()
	var gotUID, gotOID uuid.UUID
	var gotRole string
	r.GET("/x", sv.Auth(), func(c *gin.Context) {
		gotUID = GetUserID(c)
		gotOID = GetOrgID(c)
		gotRole = GetUserRole(c)
		c.Status(200)
	})
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + raw})
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, uid, gotUID)
	assert.Equal(t, oid, gotOID)
	assert.Equal(t, "admin", gotRole)
}

func TestAuthInvalidAPIKey(t *testing.T) {
	jm, _, users, _, _ := newSessionValidator(0)
	sv := NewSessionValidator(jm, users, apiKeyAuthFromStore(nil))

	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer ork_invalid"})
	assert.Equal(t, 401, w.Code)
}

func TestAuthExpiredAPIKey(t *testing.T) {
	jm, _, users, uid, oid := newSessionValidator(0)
	raw, _, hash, err := auth.GenerateAPIKey()
	require.NoError(t, err)
	expired := time.Now().Add(-time.Hour)

	sv := NewSessionValidator(jm, users, apiKeyAuthFromStore(map[string]*model.APIKey{
		hash: {
			BaseModel: model.BaseModel{ID: uuid.New()},
			UserID:    uid,
			OrgID:     oid,
			Role:      model.RoleMember,
			ExpiresAt: &expired,
		},
	}))

	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + raw})
	assert.Equal(t, 401, w.Code)
}

func TestAuthValidToken(t *testing.T) {
	jm, sv, _, uid, oid := newSessionValidator(0)
	tokens, err := jm.GenerateTokenPair(uid, oid, "admin", 0)
	require.NoError(t, err)

	r := newRouter()
	var gotUID, gotOID uuid.UUID
	var gotRole string
	r.GET("/x", sv.Auth(), func(c *gin.Context) {
		gotUID = GetUserID(c)
		gotOID = GetOrgID(c)
		gotRole = GetUserRole(c)
		c.Status(200)
	})
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, uid, gotUID)
	assert.Equal(t, oid, gotOID)
	assert.Equal(t, "admin", gotRole)
}

func TestAuthVersionMismatch(t *testing.T) {
	jm, sv, _, uid, oid := newSessionValidator(1)
	tokens, err := jm.GenerateTokenPair(uid, oid, "admin", 0)
	require.NoError(t, err)

	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	assert.Equal(t, 401, w.Code)
}

func TestAuthUserNotFound(t *testing.T) {
	jm := newJWT()
	uid, oid := uuid.New(), uuid.New()
	users := &mockUserStore{
		getByIDFn: func(context.Context, uuid.UUID) (*model.User, error) {
			return nil, sql.ErrNoRows
		},
	}
	sv := NewSessionValidator(jm, users, nil)
	tokens, err := jm.GenerateTokenPair(uid, oid, "admin", 0)
	require.NoError(t, err)

	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	assert.Equal(t, 401, w.Code)
}

func TestAuthDBError(t *testing.T) {
	jm := newJWT()
	uid, oid := uuid.New(), uuid.New()
	users := &mockUserStore{
		getByIDFn: func(context.Context, uuid.UUID) (*model.User, error) {
			return nil, errors.New("connection refused")
		},
	}
	sv := NewSessionValidator(jm, users, nil)
	tokens, err := jm.GenerateTokenPair(uid, oid, "admin", 0)
	require.NoError(t, err)

	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	assert.Equal(t, 500, w.Code)
}

func TestSessionValidatorInvalidate(t *testing.T) {
	jm, sv, users, uid, oid := newSessionValidator(0)
	tokens, err := jm.GenerateTokenPair(uid, oid, "admin", 0)
	require.NoError(t, err)

	// Prime the cache with v=0.
	r := newRouter()
	r.GET("/x", sv.Auth(), func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	require.Equal(t, 200, w.Code)

	// Bump DB version without invalidating — stale cache would still accept the token.
	users.users[uid].TokenVersion = 1
	w = doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	require.Equal(t, 200, w.Code, "stale cache should still accept token")

	// After invalidation the next request loads v=1 and rejects the old token.
	sv.Invalidate(uid)
	w = doRequest(r, "GET", "/x", map[string]string{"Authorization": "Bearer " + tokens.AccessToken})
	assert.Equal(t, 401, w.Code)
}

func TestSessionValidatorInvalidateDuringFetch(t *testing.T) {
	uid := uuid.New()
	fetching := make(chan struct{})
	releaseFetch := make(chan struct{})

	users := &blockingUserStore{
		user: &model.User{BaseModel: model.BaseModel{ID: uid}, TokenVersion: 0},
		onFetch: func() {
			select {
			case fetching <- struct{}{}:
			default:
			}
			<-releaseFetch
		},
	}
	sv := NewSessionValidator(newJWT(), users, nil)

	done := make(chan int, 1)
	go func() {
		version, err := sv.currentVersion(context.Background(), uid)
		require.NoError(t, err)
		done <- version
	}()

	<-fetching
	sv.Invalidate(uid)
	users.user.TokenVersion = 1
	close(releaseFetch)

	version := <-done
	assert.Equal(t, 1, version)

	// A stale v=0 must not have been written back into the cache.
	sv.mu.Lock()
	entry, cached := sv.cache[uid]
	sv.mu.Unlock()
	require.True(t, cached)
	assert.Equal(t, 1, entry.version)
}

type blockingUserStore struct {
	user    *model.User
	onFetch func()
}

func (m *blockingUserStore) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	if m.onFetch != nil {
		m.onFetch()
	}
	if m.user.BaseModel.ID == id {
		return m.user, nil
	}
	return nil, errors.New("not found")
}

func (m *blockingUserStore) GetByEmail(context.Context, string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *blockingUserStore) Create(context.Context, *model.User) error { return nil }

func (m *blockingUserStore) Update(context.Context, *model.User) error { return nil }

func (m *blockingUserStore) UpdatePassword(context.Context, uuid.UUID, string) error { return nil }

func (m *blockingUserStore) Update2FA(context.Context, uuid.UUID, bool, string) error { return nil }

func (m *blockingUserStore) ListByOrg(context.Context, uuid.UUID, store.ListParams) ([]model.User, int, error) {
	return nil, 0, nil
}

func (m *blockingUserStore) UpdateRole(context.Context, uuid.UUID, string) error { return nil }

func (m *blockingUserStore) RemoveFromOrg(context.Context, uuid.UUID) error { return nil }

func (m *blockingUserStore) BumpTokenVersion(context.Context, uuid.UUID) error { return nil }

func (m *blockingUserStore) Count(context.Context) (int, error) { return 0, nil }

func (m *blockingUserStore) CountByRole(context.Context, uuid.UUID, string) (int, error) {
	return 0, nil
}

func TestGettersDefaults(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, uuid.Nil, GetUserID(c))
	assert.Equal(t, uuid.Nil, GetOrgID(c))
	assert.Empty(t, GetUserRole(c))
}

// ─── RateLimit ───────────────────────────────────────────────────

func TestRateLimit(t *testing.T) {
	r := newRouter()
	r.GET("/x", RateLimit(2, time.Minute), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doRequest(r, "GET", "/x", nil).Code)
	assert.Equal(t, 200, doRequest(r, "GET", "/x", nil).Code)
	assert.Equal(t, 429, doRequest(r, "GET", "/x", nil).Code)
}

// ─── RequireRole ─────────────────────────────────────────────────

func TestRequireRoleAllowed(t *testing.T) {
	r := newRouter()
	r.GET("/x", func(c *gin.Context) { c.Set(CtxRole, "admin") }, RequireRole("admin"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doRequest(r, "GET", "/x", nil).Code)
}

func TestRequireRoleDenied(t *testing.T) {
	r := newRouter()
	r.GET("/x", func(c *gin.Context) { c.Set(CtxRole, "member") }, RequireRole("admin"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 403, doRequest(r, "GET", "/x", nil).Code)
}

// ─── WSAuth ──────────────────────────────────────────────────────

func TestWSAuthMissing(t *testing.T) {
	_, sv, _, _, _ := newSessionValidator(0)
	r := newRouter()
	r.GET("/ws", sv.WSAuth(), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 401, doRequest(r, "GET", "/ws", nil).Code)
}

func TestWSAuthQueryToken(t *testing.T) {
	jm, sv, _, uid, oid := newSessionValidator(0)
	tokens, _ := jm.GenerateTokenPair(uid, oid, "admin", 0)
	r := newRouter()
	r.GET("/ws", sv.WSAuth(), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doRequest(r, "GET", "/ws?token="+tokens.AccessToken, nil).Code)
}

func TestWSAuthHeaderFallback(t *testing.T) {
	jm, sv, _, uid, oid := newSessionValidator(0)
	tokens, _ := jm.GenerateTokenPair(uid, oid, "admin", 0)
	r := newRouter()
	r.GET("/ws", sv.WSAuth(), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doRequest(r, "GET", "/ws", map[string]string{"Authorization": "Bearer " + tokens.AccessToken}).Code)
}

func TestWSAuthInvalidToken(t *testing.T) {
	_, sv, _, _, _ := newSessionValidator(0)
	r := newRouter()
	r.GET("/ws", sv.WSAuth(), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 401, doRequest(r, "GET", "/ws?token=bad", nil).Code)
}

// ─── RequireSetupSecret ──────────────────────────────────────────

func TestRequireSetupSecretMissing(t *testing.T) {
	r := newRouter()
	r.GET("/s", RequireSetupSecret("topsecret"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 401, doRequest(r, "GET", "/s", nil).Code)
}

func TestRequireSetupSecretWrong(t *testing.T) {
	r := newRouter()
	r.GET("/s", RequireSetupSecret("topsecret"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 403, doRequest(r, "GET", "/s", map[string]string{"X-Setup-Secret": "nope"}).Code)
}

func TestRequireSetupSecretValid(t *testing.T) {
	r := newRouter()
	r.GET("/s", RequireSetupSecret("topsecret"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doRequest(r, "GET", "/s", map[string]string{"X-Setup-Secret": "topsecret"}).Code)
}

// ─── CORS ────────────────────────────────────────────────────────

func TestCORSHeaders(t *testing.T) {
	r := newRouter()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", nil)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflight(t *testing.T) {
	r := newRouter()
	r.Use(CORS())
	r.OPTIONS("/x", func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "OPTIONS", "/x", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ─── Recovery ────────────────────────────────────────────────────

func TestRecovery(t *testing.T) {
	r := newRouter()
	r.Use(Recovery(newTestLogger()))
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })
	w := doRequest(r, "GET", "/boom", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── RequestID ───────────────────────────────────────────────────

func TestRequestIDGenerated(t *testing.T) {
	r := newRouter()
	var captured string
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) {
		captured = GetRequestID(c)
		c.Status(200)
	})
	w := doRequest(r, "GET", "/x", nil)
	assert.NotEmpty(t, w.Header().Get(RequestIDKey))
	assert.Equal(t, w.Header().Get(RequestIDKey), captured)
}

func TestRequestIDPropagated(t *testing.T) {
	r := newRouter()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", map[string]string{RequestIDKey: "fixed-id"})
	assert.Equal(t, "fixed-id", w.Header().Get(RequestIDKey))
}

func TestGetRequestIDDefault(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Empty(t, GetRequestID(c))
}

// ─── Logger ──────────────────────────────────────────────────────

func TestLoggerMiddleware(t *testing.T) {
	r := newRouter()
	r.Use(Logger(newTestLogger()))
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	assert.Equal(t, 200, doRequest(r, "GET", "/x?foo=bar", nil).Code)
}

func TestLoggerMiddlewareError(t *testing.T) {
	r := newRouter()
	r.Use(Logger(newTestLogger()))
	r.GET("/x", func(c *gin.Context) {
		_ = c.Error(assertErr{})
		c.String(500, "err")
	})
	assert.Equal(t, 500, doRequest(r, "GET", "/x", nil).Code)
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }

// ─── Branding ────────────────────────────────────────────────────

func TestBranding(t *testing.T) {
	r := newRouter()
	r.Use(Branding())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	w := doRequest(r, "GET", "/x", nil)
	assert.NotEmpty(t, w.Header().Get("X-Powered-By"))
}

// ─── Sentry ──────────────────────────────────────────────────────

func TestSentryNoop(t *testing.T) {
	r := newRouter()
	r.Use(Sentry())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doRequest(r, "GET", "/x", nil).Code)
}
