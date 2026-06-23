package apierr

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProblemDetailError(t *testing.T) {
	p := ErrBadRequest.WithDetail("boom")
	assert.Equal(t, "boom", p.Error())
}

func TestWithDetailDoesNotMutateOriginal(t *testing.T) {
	orig := ErrNotFound
	cp := orig.WithDetail("missing")
	assert.Equal(t, "missing", cp.Detail)
	assert.Empty(t, orig.Detail, "original must stay unmodified")
	assert.Equal(t, http.StatusNotFound, cp.Status)
}

func TestWithInstance(t *testing.T) {
	p := ErrInternal.WithInstance("req-123")
	assert.Equal(t, "req-123", p.Instance)
	assert.Empty(t, ErrInternal.Instance)
}

func TestWithFieldErrors(t *testing.T) {
	errs := []FieldError{{Field: "name", Message: "required"}}
	p := ErrValidation.WithFieldErrors(errs)
	assert.Equal(t, errs, p.Errors)
	assert.Nil(t, ErrValidation.Errors)
}

func TestPredefinedErrorStatuses(t *testing.T) {
	cases := map[*ProblemDetail]int{
		ErrBadRequest:      http.StatusBadRequest,
		ErrValidation:      http.StatusBadRequest,
		ErrUnauthorized:    http.StatusUnauthorized,
		ErrForbidden:       http.StatusForbidden,
		ErrNotFound:        http.StatusNotFound,
		ErrConflict:        http.StatusConflict,
		ErrInternal:        http.StatusInternalServerError,
		ErrTooManyRequests: http.StatusTooManyRequests,
	}
	for pd, status := range cases {
		assert.Equal(t, status, pd.Status)
		assert.NotEmpty(t, pd.Title)
		assert.NotEmpty(t, pd.Type)
	}
}
