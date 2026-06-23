package v1

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

func newWebhookHandler(fs *testsupport.FakeStore) *WebhookHandler {
	orch := testsupport.NewFakeOrchestrator()
	build := service.NewBuildService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testLogger())
	settings := service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger())
	notif := service.NewNotificationService(fs, settings, testLogger())
	ra := service.NewRegistryAuth(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testLogger())
	dep := service.NewDeployService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), build, notif, ra, testsupport.NewFakeEnqueuer())
	return NewWebhookHandler(fs, dep, testsupport.NewProviders(fs), testLogger())
}

func TestWebhookGitHubBadID(t *testing.T) {
	h := newWebhookHandler(testsupport.NewFakeStore())
	r := gin.New()
	r.POST("/wh/github/:appId", h.GitHub)
	assert.Equal(t, 400, doJSON(r, "POST", "/wh/github/bad", nil).Code)
}

func TestWebhookGitHubPing(t *testing.T) {
	h := newWebhookHandler(testsupport.NewFakeStore())
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "ping")
		h.GitHub(c)
	})
	assert.Equal(t, 200, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), nil).Code)
}

func TestWebhookGitHubIgnoredEvent(t *testing.T) {
	h := newWebhookHandler(testsupport.NewFakeStore())
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "issues")
		h.GitHub(c)
	})
	assert.Equal(t, 200, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), nil).Code)
}

func TestWebhookGitHubAppNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, context.Canceled
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "push")
		h.GitHub(c)
	})
	assert.Equal(t, 404, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), map[string]any{"ref": "refs/heads/main"}).Code)
}

func TestWebhookGitHubAutoDeployDisabled(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{AutoDeploy: false}, nil
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "push")
		h.GitHub(c)
	})
	assert.Equal(t, 200, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), map[string]any{"ref": "refs/heads/main"}).Code)
}

func TestWebhookGitHubBranchMismatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{AutoDeploy: true, GitBranch: "main"}, nil
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "push")
		h.GitHub(c)
	})
	assert.Equal(t, 200, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), map[string]any{"ref": "refs/heads/dev"}).Code)
}

func TestWebhookGitHubWatchPathsNoMatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{AutoDeploy: true, GitBranch: "main", WatchPaths: []string{"backend/"}}, nil
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "push")
		h.GitHub(c)
	})
	body := map[string]any{
		"ref":     "refs/heads/main",
		"commits": []map[string]any{{"modified": []string{"frontend/app.js"}}},
	}
	assert.Equal(t, 200, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), body).Code)
}

func TestWebhookGitHubMissingSignature(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{AutoDeploy: true, GitBranch: "main", WebhookSecret: "s3cr3t"}, nil
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/github/:appId", func(c *gin.Context) {
		c.Request.Header.Set("X-GitHub-Event", "push")
		h.GitHub(c)
	})
	// Secret configured but no X-Hub-Signature-256 header → must be rejected.
	assert.Equal(t, 401, doJSON(r, "POST", "/wh/github/"+uuid.New().String(), map[string]any{"ref": "refs/heads/main"}).Code)
}

func TestWebhookGitLabBadID(t *testing.T) {
	h := newWebhookHandler(testsupport.NewFakeStore())
	r := gin.New()
	r.POST("/wh/gitlab/:appId", h.GitLab)
	assert.Equal(t, 400, doJSON(r, "POST", "/wh/gitlab/bad", nil).Code)
}

func TestWebhookGitLabInvalidToken(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{AutoDeploy: true, WebhookSecret: "s3cr3t"}, nil
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/gitlab/:appId", h.GitLab)
	assert.Equal(t, 401, doJSON(r, "POST", "/wh/gitlab/"+uuid.New().String(), map[string]any{}).Code)
}

func TestWebhookGitLabBranchMismatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{AutoDeploy: true, GitBranch: "main"}, nil
	}
	h := newWebhookHandler(fs)
	r := gin.New()
	r.POST("/wh/gitlab/:appId", h.GitLab)
	assert.Equal(t, 200, doJSON(r, "POST", "/wh/gitlab/"+uuid.New().String(), map[string]any{"ref": "refs/heads/dev"}).Code)
}

func TestMatchPath(t *testing.T) {
	assert.True(t, matchPath("src/main.go", "src/"))
	assert.True(t, matchPath("apps/api/x.go", "apps/api/**"))
	assert.True(t, matchPath("main.go", "*.go"))
	assert.True(t, matchPath("dir/file.txt", "dir"))
	assert.False(t, matchPath("other/file.txt", "dir"))
	assert.False(t, matchPath("readme.md", "*.go"))
}
