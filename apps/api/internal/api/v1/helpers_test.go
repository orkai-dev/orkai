package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
)

func init() { gin.SetMode(gin.TestMode) }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testJWT() *auth.JWTManager {
	return auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
}

// authMiddleware injects auth context (user/org/role) for handler tests.
func authMiddleware(userID, orgID uuid.UUID, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID)
		c.Set(middleware.CtxOrgID, orgID)
		c.Set(middleware.CtxRole, role)
		c.Next()
	}
}

// newAuthedRouter returns a gin engine that injects an admin auth context.
func newAuthedRouter() (*gin.Engine, uuid.UUID, uuid.UUID) {
	uid, oid := uuid.New(), uuid.New()
	r := gin.New()
	r.Use(authMiddleware(uid, oid, "admin"))
	return r, uid, oid
}

func doJSON(r http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
