package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateListAndGet(t *testing.T) {
	fs := testsupport.NewFakeStore()
	tid := uuid.New()
	fs.TemplatesStore.ListFn = func(ctx context.Context, params store.ListParams) ([]model.Template, int, error) {
		return []model.Template{{}}, 1, nil
	}
	fs.TemplatesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Template, error) {
		return &model.Template{BaseModel: model.BaseModel{ID: tid}}, nil
	}
	s := NewTemplateService(fs, testsupport.NewTestLogger())

	list, total, err := s.List(context.Background(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)

	got, err := s.GetByID(context.Background(), tid)
	require.NoError(t, err)
	assert.Equal(t, tid, got.ID)
}
