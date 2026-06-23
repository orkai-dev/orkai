package service

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyResourceDeletedNilReceiver(t *testing.T) {
	var s *NotificationService
	// Must not panic when the service is absent.
	assert.NotPanics(t, func() {
		s.NotifyResourceDeleted(uuid.New(), model.EventAppDeleted, "api", "detail")
	})
}

func TestNotifyResourceDeletedSends(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()

	var mu sync.Mutex
	var gotOrg uuid.UUID
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, id uuid.UUID) ([]model.NotificationChannel, error) {
		mu.Lock()
		gotOrg = id
		mu.Unlock()
		return nil, nil
	}

	s := newNotificationService(fs)
	s.NotifyResourceDeleted(orgID, model.EventAppDeleted, "api", "detail")
	s.Shutdown()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, orgID, gotOrg)
}

func TestNotifyProjectResourceDeletedResolvesOrg(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		assert.Equal(t, projectID, id)
		return &model.Project{OrgID: orgID}, nil
	}

	var mu sync.Mutex
	var gotOrg uuid.UUID
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, id uuid.UUID) ([]model.NotificationChannel, error) {
		mu.Lock()
		gotOrg = id
		mu.Unlock()
		return nil, nil
	}

	s := newNotificationService(fs)
	notifyProjectResourceDeleted(s, fs, testsupport.NewTestLogger(), projectID, model.EventAppDeleted, "api", "detail")
	s.Shutdown()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, orgID, gotOrg)
}

func TestNotifyProjectResourceDeletedNilNotif(t *testing.T) {
	fs := testsupport.NewFakeStore()
	called := false
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		called = true
		return &model.Project{}, nil
	}
	// nil notif must short-circuit before touching the store.
	notifyProjectResourceDeleted(nil, fs, testsupport.NewTestLogger(), uuid.New(), model.EventAppDeleted, "api", "detail")
	require.False(t, called)
}
