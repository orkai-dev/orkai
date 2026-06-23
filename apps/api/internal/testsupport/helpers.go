package testsupport

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// NewTestLogger returns a slog.Logger that discards all output.
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// NewGinContext returns a gin.Context bound to a fresh ResponseRecorder for
// handler-level unit tests, along with the recorder.
func NewGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// JSONRequest builds an *http.Request with a JSON body for the given method/target.
func JSONRequest(method, target string, body any) *http.Request {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// RawRequest builds an *http.Request with a raw string body.
func RawRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// SetParam sets a URL path parameter on a gin.Context.
func SetParam(c *gin.Context, key, value string) {
	c.Params = append(c.Params, gin.Param{Key: key, Value: value})
}

// AuthInfo bundles the identity values stored on a gin.Context by the auth
// middleware.
type AuthInfo struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   string
}

// SetAuthContext stores user/org/role on the context, mimicking the auth
// middleware so handlers reading GetUserID/GetOrgID/GetUserRole work.
func SetAuthContext(c *gin.Context, info AuthInfo) {
	c.Set(middleware.CtxUserID, info.UserID)
	c.Set(middleware.CtxOrgID, info.OrgID)
	c.Set(middleware.CtxRole, info.Role)
}

// DecodeJSON unmarshals a recorder body into the provided destination.
func DecodeJSON(body *bytes.Buffer, dst any) error {
	return json.Unmarshal(body.Bytes(), dst)
}
