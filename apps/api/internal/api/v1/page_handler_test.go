package v1

import (
	"context"
	"testing"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

func newPageHandler(fs *testsupport.FakeStore) *PageHandler {
	return NewPageHandler(
		service.NewPageService(fs, pages.NewRegistry(), testLogger(), nil),
		nil,
		fs,
		service.NewAuthz(fs),
	)
}

func TestPageListAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	var gotFilter store.PageListFilter
	fs.PagesStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.PageListFilter) ([]model.Page, int, error) {
		gotFilter = f
		return []model.Page{{}}, 1, nil
	}
	h := newPageHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/pages", h.ListAll)

	assert.Equal(t, 200, doJSON(r, "GET", "/pages", nil).Code)
	assert.Equal(t, "", gotFilter.Provider)

	gotFilter = store.PageListFilter{}
	assert.Equal(t, 200, doJSON(r, "GET", "/pages?provider=cloudflare_pages", nil).Code)
	assert.Equal(t, string(model.PageProviderCloudflarePages), gotFilter.Provider)

	assert.Equal(t, 400, doJSON(r, "GET", "/pages?provider=foo", nil).Code)
}
