package httputil

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/stretchr/testify/assert"
)

func init() { gin.SetMode(gin.TestMode) }

func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	return c, w
}

func TestNewListResponse(t *testing.T) {
	resp := NewListResponse([]int{1, 2, 3}, 1, 2, 5)
	assert.Equal(t, []int{1, 2, 3}, resp.Items)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 2, resp.Pagination.PerPage)
	assert.Equal(t, 5, resp.Pagination.Total)
	assert.Equal(t, 3, resp.Pagination.TotalPages) // 5/2 = 2, +1 remainder
}

func TestNewListResponseExactPages(t *testing.T) {
	resp := NewListResponse([]int{1, 2}, 1, 2, 4)
	assert.Equal(t, 2, resp.Pagination.TotalPages) // 4/2 = 2, no remainder
}

func TestNewListResponseNilItems(t *testing.T) {
	resp := NewListResponse[int](nil, 1, 10, 0)
	assert.NotNil(t, resp.Items)
	assert.Equal(t, []int{}, resp.Items)
	assert.Equal(t, 0, resp.Pagination.TotalPages)
}

func TestRespondOK(t *testing.T) {
	c, w := newCtx()
	RespondOK(c, gin.H{"ok": true})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "true")
}

func TestRespondList(t *testing.T) {
	c, w := newCtx()
	RespondList(c, []string{"a"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "a")
}

func TestRespondListNil(t *testing.T) {
	c, w := newCtx()
	RespondList[string](c, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
}

func TestRespondCreatedWithLocation(t *testing.T) {
	c, w := newCtx()
	RespondCreated(c, gin.H{"id": 1}, "/things/1")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "/things/1", w.Header().Get("Location"))
}

func TestRespondCreatedNoLocation(t *testing.T) {
	c, w := newCtx()
	RespondCreated(c, gin.H{"id": 1}, "")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
}

func TestRespondAccepted(t *testing.T) {
	c, w := newCtx()
	RespondAccepted(c, gin.H{"queued": true})
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestRespondNoContent(t *testing.T) {
	c, _ := newCtx()
	RespondNoContent(c)
	// gin's c.Status defers writing the header until flush, so assert on the
	// gin ResponseWriter's recorded status rather than the recorder code.
	assert.Equal(t, http.StatusNoContent, c.Writer.Status())
}

func TestRespondErrorProblemDetail(t *testing.T) {
	c, w := newCtx()
	RespondError(c, apierr.ErrNotFound.WithDetail("nope"))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "nope")
}

func TestRespondErrorGenericError(t *testing.T) {
	c, w := newCtx()
	RespondError(c, errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "unexpected")
}
