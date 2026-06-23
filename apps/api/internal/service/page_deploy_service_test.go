package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
)

// fakeBuildTarget is a DeployTarget whose StaticSiteBuilder.BuildStatic is
// controllable, so the build branch of RunJob can be tested without a cluster.
type fakeBuildTarget struct {
	*orchestrator.NoopTarget
	fn func(ctx context.Context, opts orchestrator.StaticBuildOpts) (*orchestrator.StaticBuildResult, error)
}

func (f *fakeBuildTarget) BuildStatic(ctx context.Context, opts orchestrator.StaticBuildOpts) (*orchestrator.StaticBuildResult, error) {
	return f.fn(ctx, opts)
}

// seedBuildPage seeds a project + cloud account and a build-enabled page, and
// returns the store, page, and a fresh queued deployment.
func seedBuildPage(t *testing.T, logger *slog.Logger) (*testsupport.FakeStore, *model.Page, *model.PageDeployment) {
	t.Helper()
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	projectID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}
	cloudID := uuid.New()
	cfg, _ := json.Marshal(pages.Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret", DefaultRegion: "us-east-1"})
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: cloudID}, OrgID: orgID, Type: model.ResourceCloudAccount, Config: cfg}, nil
	}
	page := &model.Page{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		ProjectID:      projectID,
		Name:           "build-page",
		GitRepo:        "https://github.com/acme/site",
		GitBranch:      "main",
		BuildEnabled:   true,
		PackageManager: "auto",
		OutputDir:      "dist",
		RootDirectory:  ".",
		Provider:       model.PageProviderAWSCloudFront,
		CloudAccountID: &cloudID,
		Region:         "us-east-1",
		Runtime:        &model.PageRuntime{},
		Status:         model.PageStatusIdle,
	}
	must(t, fs.PagesStore.Create(context.Background(), page))
	dep := &model.PageDeployment{PageID: page.ID, Status: model.PageDeployQueued}
	must(t, fs.PageDeploymentsStore.Create(context.Background(), dep))
	return fs, page, dep
}

func TestPageDeploy_BuildEnabled_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fs, _, dep := seedBuildPage(t, logger)

	// The "build" produces a directory of static files for the provider to sync.
	builtDir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(builtDir, "index.html"), []byte("<h1>built</h1>"), 0o644))

	var gotOpts orchestrator.StaticBuildOpts
	target := &fakeBuildTarget{
		NoopTarget: orchestrator.NewNoopTarget(uuid.New(), logger),
		fn: func(ctx context.Context, opts orchestrator.StaticBuildOpts) (*orchestrator.StaticBuildResult, error) {
			gotOpts = opts
			return &orchestrator.StaticBuildResult{FilesDir: builtDir, CommitSHA: "abc123", Cleanup: func() {}}, nil
		},
	}
	reg, err := orchestrator.NewTargetRegistry(uuid.Nil, target)
	must(t, err)

	fake := &fakePagesProvider{}
	registry := pages.NewRegistry(fake)
	prov := providers.New(fs.SettingsStore, logger)
	publishSvc := NewPagePublishService(fs, prov, logger)
	svc := NewPageDeployService(fs, registry, publishSvc, nil, nil, reg, logger)

	if err := svc.RunJob(context.Background(), dep.ID); err != nil {
		t.Fatalf("RunJob: %v", err)
	}

	if gotOpts.OutputDir != "dist" || gotOpts.PackageManager != "auto" {
		t.Errorf("build opts = %+v, want OutputDir=dist PackageManager=auto", gotOpts)
	}
	if _, ok := fake.deployedFiles["index.html"]; !ok {
		t.Errorf("expected built index.html to be deployed, got %v", keys(fake.deployedFiles))
	}
	gotDep, _ := fs.PageDeploymentsStore.GetByID(context.Background(), dep.ID)
	if gotDep.Status != model.PageDeploySuccess {
		t.Errorf("deployment status = %q, want success", gotDep.Status)
	}
	if gotDep.CommitSHA != "abc123" {
		t.Errorf("commit sha = %q, want abc123", gotDep.CommitSHA)
	}
}

func TestPageDeploy_BuildEnabled_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fs, _, dep := seedBuildPage(t, logger)

	target := &fakeBuildTarget{
		NoopTarget: orchestrator.NewNoopTarget(uuid.New(), logger),
		fn: func(ctx context.Context, opts orchestrator.StaticBuildOpts) (*orchestrator.StaticBuildResult, error) {
			return &orchestrator.StaticBuildResult{Logs: "boom"}, errors.New("build script failed")
		},
	}
	reg, err := orchestrator.NewTargetRegistry(uuid.Nil, target)
	must(t, err)

	fake := &fakePagesProvider{}
	registry := pages.NewRegistry(fake)
	prov := providers.New(fs.SettingsStore, logger)
	publishSvc := NewPagePublishService(fs, prov, logger)
	svc := NewPageDeployService(fs, registry, publishSvc, nil, nil, reg, logger)

	if err := svc.RunJob(context.Background(), dep.ID); err != nil {
		t.Fatalf("RunJob returned error (should record failure, not propagate): %v", err)
	}

	if fake.deployCount != 0 {
		t.Errorf("provider.Deploy must not be called when the build fails (got %d)", fake.deployCount)
	}
	gotDep, _ := fs.PageDeploymentsStore.GetByID(context.Background(), dep.ID)
	if gotDep.Status != model.PageDeployFailed {
		t.Errorf("deployment status = %q, want failed", gotDep.Status)
	}
}

// fakePagesProvider records what Deploy receives so the test can assert that the
// clone → resolve publish_path → Deploy(filesDir) path hands off the right files
// without touching AWS.
type fakePagesProvider struct {
	deployedFiles map[string]string // S3 key -> contents
	provisioned   bool
	deployCount   int
	deleteCalled  bool
}

func (f *fakePagesProvider) Name() string { return string(model.PageProviderAWSCloudFront) }

func (f *fakePagesProvider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	return true, "ok", nil
}

func (f *fakePagesProvider) Provision(ctx context.Context, page *model.Page, cfg json.RawMessage, tags map[string]string, save pages.SaveRuntime) (*model.PageRuntime, error) {
	f.provisioned = true
	rt := &model.PageRuntime{
		BucketName:     "fake-bucket",
		DistributionID: "FAKEDIST",
		DefaultURL:     "https://fake.cloudfront.net",
	}
	_ = save(ctx, rt)
	return rt, nil
}

func (f *fakePagesProvider) Deploy(ctx context.Context, page *model.Page, cfg json.RawMessage, filesDir string, onLog func(string)) (*pages.DeployResult, error) {
	f.deployCount++
	f.deployedFiles = map[string]string{}
	_ = filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(filesDir, path)
		data, _ := os.ReadFile(path)
		f.deployedFiles[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	return &pages.DeployResult{ProviderRef: "INVAL123", DefaultURL: page.Runtime.DefaultURL}, nil
}

func (f *fakePagesProvider) Delete(ctx context.Context, page *model.Page, cfg json.RawMessage) error {
	f.deleteCalled = true
	return nil
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// makeRepo creates a local git repo with output/index.html and returns its path.
func makeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	out := filepath.Join(repo, "output")
	if err := os.MkdirAll(filepath.Join(out, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(filepath.Join(out, "index.html"), []byte("<h1>hi</h1>"), 0o644))
	must(t, os.WriteFile(filepath.Join(out, "assets", "app.js"), []byte("console.log(1)"), 0o644))
	// A file OUTSIDE the publish folder must NOT be deployed.
	must(t, os.WriteFile(filepath.Join(repo, "README.md"), []byte("readme"), 0o644))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	git(t, repo, "branch", "-M", "trunk")
	return repo
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPageDeploy_PublishFolderHandoff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := makeRepo(t)

	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Seed project + cloud_account resource + page.
	orgID := uuid.New()
	projectID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}
	cloudID := uuid.New()
	cfg, _ := json.Marshal(pages.Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret", DefaultRegion: "us-east-1"})
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: cloudID}, OrgID: orgID, Type: model.ResourceCloudAccount, Config: cfg}, nil
	}

	page := &model.Page{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		ProjectID:      projectID,
		Name:           "my-page",
		GitRepo:        repo, // local path clone
		GitBranch:      "trunk",
		PublishPath:    "output",
		Provider:       model.PageProviderAWSCloudFront,
		CloudAccountID: &cloudID,
		Region:         "us-east-1",
		Runtime:        &model.PageRuntime{},
		Status:         model.PageStatusIdle,
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	dep := &model.PageDeployment{PageID: page.ID, Status: model.PageDeployQueued}
	must(t, fs.PageDeploymentsStore.Create(context.Background(), dep))

	fake := &fakePagesProvider{}
	registry := pages.NewRegistry(fake)
	prov := providers.New(fs.SettingsStore, logger)
	publishSvc := NewPagePublishService(fs, prov, logger)
	svc := NewPageDeployService(fs, registry, publishSvc, nil, nil, nil, logger)

	if err := svc.RunJob(context.Background(), dep.ID); err != nil {
		t.Fatalf("RunJob: %v", err)
	}

	if !fake.provisioned {
		t.Error("expected provider.Provision to be called")
	}
	// Only files under output/ should be handed to Deploy, rooted at the folder.
	if _, ok := fake.deployedFiles["index.html"]; !ok {
		t.Errorf("expected index.html to be deployed, got keys: %v", keys(fake.deployedFiles))
	}
	if _, ok := fake.deployedFiles["assets/app.js"]; !ok {
		t.Errorf("expected assets/app.js to be deployed, got keys: %v", keys(fake.deployedFiles))
	}
	if _, ok := fake.deployedFiles["README.md"]; ok {
		t.Error("README.md is outside the publish folder and must not be deployed")
	}

	// Page should be marked live and deployment success.
	got, _ := fs.PagesStore.GetByID(context.Background(), page.ID)
	if got.Status != model.PageStatusLive {
		t.Errorf("page status = %q, want live", got.Status)
	}
	gotDep, _ := fs.PageDeploymentsStore.GetByID(context.Background(), dep.ID)
	if gotDep.Status != model.PageDeploySuccess {
		t.Errorf("deployment status = %q, want success", gotDep.Status)
	}
}

// makeRepoTwoFolders creates a local git repo with v1/ and v2/ folders in a
// single commit and returns its path.
func makeRepoTwoFolders(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	for _, f := range []struct{ dir, body string }{{"v1", "<h1>one</h1>"}, {"v2", "<h1>two</h1>"}} {
		must(t, os.MkdirAll(filepath.Join(repo, f.dir), 0o755))
		must(t, os.WriteFile(filepath.Join(repo, f.dir, "index.html"), []byte(f.body), 0o644))
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	git(t, repo, "branch", "-M", "trunk")
	return repo
}

// TestPageDeploy_PublishPathChangeBypassesDedup guards the content-correctness
// bug where changing publish_path without a new commit was silently skipped by
// the commit-SHA-only dedup, leaving the CDN serving the old folder.
func TestPageDeploy_PublishPathChangeBypassesDedup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := makeRepoTwoFolders(t)

	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	orgID := uuid.New()
	projectID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}
	cloudID := uuid.New()
	cfg, _ := json.Marshal(pages.Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret", DefaultRegion: "us-east-1"})
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: cloudID}, OrgID: orgID, Type: model.ResourceCloudAccount, Config: cfg}, nil
	}

	page := &model.Page{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		ProjectID:      projectID,
		Name:           "my-page",
		GitRepo:        repo,
		GitBranch:      "trunk",
		PublishPath:    "v1",
		Provider:       model.PageProviderAWSCloudFront,
		CloudAccountID: &cloudID,
		Region:         "us-east-1",
		Runtime:        &model.PageRuntime{},
		Status:         model.PageStatusIdle,
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// Faithful dedup lookup: return the most recent successful deployment for the
	// page (the in-memory default returns "not found").
	fs.PageDeploymentsStore.GetLatestSuccessFn = func(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error) {
		deps, _, _ := fs.PageDeploymentsStore.ListByPage(ctx, pageID, store.ListParams{Page: 1, PerPage: 1000})
		var latest *model.PageDeployment
		for i := range deps {
			d := deps[i]
			if d.Status != model.PageDeploySuccess {
				continue
			}
			if latest == nil || d.CreatedAt.After(latest.CreatedAt) {
				latest = &deps[i]
			}
		}
		if latest == nil {
			return nil, errors.New("not found")
		}
		return latest, nil
	}

	fake := &fakePagesProvider{}
	registry := pages.NewRegistry(fake)
	prov := providers.New(fs.SettingsStore, logger)
	publishSvc := NewPagePublishService(fs, prov, logger)
	svc := NewPageDeployService(fs, registry, publishSvc, nil, nil, nil, logger)

	base := time.Now()
	seq := 0
	runDeploy := func() *model.PageDeployment {
		t.Helper()
		seq++
		dep := &model.PageDeployment{
			BaseModel: model.BaseModel{CreatedAt: base.Add(time.Duration(seq) * time.Second)},
			PageID:    page.ID,
			Status:    model.PageDeployQueued,
		}
		must(t, fs.PageDeploymentsStore.Create(context.Background(), dep))
		if err := svc.RunJob(context.Background(), dep.ID); err != nil {
			t.Fatalf("RunJob: %v", err)
		}
		got, _ := fs.PageDeploymentsStore.GetByID(context.Background(), dep.ID)
		return got
	}

	// First deploy of v1.
	d1 := runDeploy()
	if d1.Status != model.PageDeploySuccess {
		t.Fatalf("first deploy status = %q, want success", d1.Status)
	}
	if fake.deployCount != 1 {
		t.Fatalf("expected 1 sync after first deploy, got %d", fake.deployCount)
	}
	if got := fake.deployedFiles["index.html"]; got != "<h1>one</h1>" {
		t.Fatalf("first deploy synced %q, want v1 content", got)
	}

	// Repoint publish_path to v2 WITHOUT a new commit, then redeploy. The
	// commit SHA is identical; a commit-only dedup would wrongly skip.
	page.PublishPath = "v2"
	must(t, fs.PagesStore.UpdateSettings(context.Background(), page))

	d2 := runDeploy()
	if d2.Status != model.PageDeploySuccess {
		t.Fatalf("second deploy status = %q, want success", d2.Status)
	}
	if fake.deployCount != 2 {
		t.Fatalf("publish_path change must re-sync: expected 2 syncs, got %d", fake.deployCount)
	}
	if got := fake.deployedFiles["index.html"]; got != "<h1>two</h1>" {
		t.Fatalf("second deploy synced %q, want v2 content", got)
	}

	// A redeploy with the SAME commit AND SAME publish_path must still dedup.
	d3 := runDeploy()
	if d3.Status != model.PageDeploySuccess {
		t.Fatalf("third deploy status = %q, want success", d3.Status)
	}
	if fake.deployCount != 2 {
		t.Fatalf("unchanged commit+publish_path must skip sync: expected 2 syncs, got %d", fake.deployCount)
	}
}

// TestPageUpdate_BlocksCloudTargetChangeWhileDeploying guards the provisioning
// window between TryMarkDeploying and the first runtime save: status is
// "deploying" but Runtime is still empty, so a BucketName-only guard would let
// a PATCH retarget the page's cloud account or region mid-provision.
func TestPageUpdate_BlocksCloudTargetChangeWhileDeploying(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	orgID := uuid.New()
	projectID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}

	oldCloud := uuid.New()
	page := &model.Page{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		ProjectID:      projectID,
		Name:           "my-page",
		Provider:       model.PageProviderAWSCloudFront,
		CloudAccountID: &oldCloud,
		Region:         "us-east-1",
		Runtime:        &model.PageRuntime{}, // mid-provision: no IDs recorded yet
		Status:         model.PageStatusDeploying,
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	svc := NewPageService(fs, pages.NewRegistry(), logger, nil)

	newCloud := uuid.New()
	if _, err := svc.Update(context.Background(), page.ID, UpdatePageInput{CloudAccountID: &newCloud}); err == nil {
		t.Error("expected cloud account change to be rejected during an in-flight deploy")
	}

	newRegion := "eu-west-1"
	if _, err := svc.Update(context.Background(), page.ID, UpdatePageInput{Region: &newRegion}); err == nil {
		t.Error("expected region change to be rejected during an in-flight deploy")
	}

	// The persisted cloud target must be untouched.
	got, _ := fs.PagesStore.GetByID(context.Background(), page.ID)
	if got.CloudAccountID == nil || *got.CloudAccountID != oldCloud {
		t.Errorf("cloud account changed mid-provision: %v", got.CloudAccountID)
	}
	if got.Region != "us-east-1" {
		t.Errorf("region changed mid-provision: %q", got.Region)
	}
}

// TestPageUpdate_CloudTargetChangeRacesDeploy covers the TOCTOU window: the
// in-memory guard reads a non-deploying snapshot, but a deploy starts before the
// write. The atomic "status <> deploying" guard on the persistence must reject
// the cloud-target change so the page can't end up pointing at a new account
// while the worker provisions into the old one.
func TestPageUpdate_CloudTargetChangeRacesDeploy(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	orgID := uuid.New()
	projectID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}
	oldCloud := uuid.New()
	newCloud := uuid.New()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: id}, OrgID: orgID, Type: model.ResourceCloudAccount}, nil
	}

	pageID := uuid.New()
	page := &model.Page{
		BaseModel:      model.BaseModel{ID: pageID},
		ProjectID:      projectID,
		Name:           "my-page",
		Provider:       model.PageProviderAWSCloudFront,
		CloudAccountID: &oldCloud,
		Region:         "us-east-1",
		Runtime:        &model.PageRuntime{},
		Status:         model.PageStatusDeploying, // the live row: a deploy is running
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// The handler's snapshot was taken just before the deploy started, so it
	// still looks idle and lets the in-memory guard pass.
	fs.PagesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Page, error) {
		snap := *page
		snap.Status = model.PageStatusIdle
		snap.Runtime = &model.PageRuntime{}
		return &snap, nil
	}

	svc := NewPageService(fs, pages.NewRegistry(), logger, nil)
	if _, err := svc.Update(context.Background(), pageID, UpdatePageInput{CloudAccountID: &newCloud}); err == nil {
		t.Fatal("expected the atomic guard to reject a cloud account change that raced a deploy")
	}

	fs.PagesStore.GetByIDFn = nil // read the real record
	got, _ := fs.PagesStore.GetByID(context.Background(), pageID)
	if got.CloudAccountID == nil || *got.CloudAccountID != oldCloud {
		t.Errorf("cloud account was retargeted despite a racing deploy: %v", got.CloudAccountID)
	}
}

// TestPageDelete_RacesDeploy covers the Delete TOCTOU window: the snapshot looks
// deletable, but a deploy starts before the row is removed. The atomic guard
// must refuse to delete (and skip teardown) so AWS resources aren't orphaned.
func TestPageDelete_RacesDeploy(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pageID := uuid.New()
	page := &model.Page{
		BaseModel: model.BaseModel{ID: pageID},
		ProjectID: uuid.New(),
		Name:      "my-page",
		Provider:  model.PageProviderAWSCloudFront,
		Runtime:   &model.PageRuntime{BucketName: "prod-bucket"},
		Status:    model.PageStatusDeploying, // the live row: a deploy is running
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// Snapshot predates the deploy, so the fast-path pre-check passes.
	fs.PagesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Page, error) {
		snap := *page
		snap.Status = model.PageStatusLive
		return &snap, nil
	}

	svc := NewPageService(fs, pages.NewRegistry(), logger, nil)
	if err := svc.Delete(context.Background(), pageID); err == nil {
		t.Fatal("expected delete to refuse when a deploy started after the snapshot read")
	}

	fs.PagesStore.GetByIDFn = nil
	if _, gerr := fs.PagesStore.GetByID(context.Background(), pageID); gerr != nil {
		t.Fatalf("page row was deleted despite a racing deploy: %v", gerr)
	}
}

// TestPageDelete_TeardownRunsDetached verifies cloud teardown is detached from
// the request context: even when the caller's context is already cancelled (a
// disconnected client), the teardown still runs to completion against AWS.
func TestPageDelete_TeardownRunsDetached(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	orgID := uuid.New()
	cloudID := uuid.New()
	cfg, _ := json.Marshal(pages.Credentials{AccessKeyID: "AKIA", SecretAccessKey: "secret", DefaultRegion: "us-east-1"})
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: cloudID}, OrgID: orgID, Type: model.ResourceCloudAccount, Config: cfg}, nil
	}

	fake := &fakePagesProvider{}
	svc := NewPageService(fs, pages.NewRegistry(fake), logger, nil)

	pageID := uuid.New()
	page := &model.Page{
		BaseModel:      model.BaseModel{ID: pageID},
		Name:           "my-page",
		Provider:       model.PageProviderAWSCloudFront,
		CloudAccountID: &cloudID,
		Region:         "us-east-1",
		Status:         model.PageStatusLive,
		Runtime:        &model.PageRuntime{BucketName: "prod-bucket", DistributionID: "DIST123"},
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// A disconnected client: the request context is already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := svc.Delete(ctx, pageID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	svc.WaitForTeardowns()

	if !fake.deleteCalled {
		t.Error("expected detached teardown to call provider.Delete despite a cancelled request context")
	}
	if _, gerr := fs.PagesStore.GetByID(context.Background(), pageID); gerr == nil {
		t.Error("expected page row to be removed")
	}
}

// TestPageDelete_BlockedWhileDeploying guards the orphan-on-delete window: a
// page deleted mid-provision (status=deploying, runtime not yet recorded) would
// drop the only record of the AWS resources the worker is creating.
func TestPageDelete_BlockedWhileDeploying(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	page := &model.Page{
		BaseModel: model.BaseModel{ID: uuid.New()},
		ProjectID: uuid.New(),
		Name:      "my-page",
		Provider:  model.PageProviderAWSCloudFront,
		Runtime:   &model.PageRuntime{}, // mid-provision: IDs not yet committed
		Status:    model.PageStatusDeploying,
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	svc := NewPageService(fs, pages.NewRegistry(), logger, nil)

	if err := svc.Delete(context.Background(), page.ID); err == nil {
		t.Fatal("expected Delete to be rejected while the page is deploying")
	}

	// The row must survive so the worker can still record its AWS resource IDs.
	if _, gerr := fs.PagesStore.GetByID(context.Background(), page.ID); gerr != nil {
		t.Fatalf("page row was deleted despite an in-flight deploy: %v", gerr)
	}

	// Once the deploy settles, delete must proceed normally.
	must(t, fs.PagesStore.UpdateStatus(context.Background(), page.ID, model.PageStatusLive))
	if err := svc.Delete(context.Background(), page.ID); err != nil {
		t.Fatalf("Delete after deploy finished: %v", err)
	}
	if _, gerr := fs.PagesStore.GetByID(context.Background(), page.ID); gerr == nil {
		t.Fatal("expected page row to be removed after a non-deploying delete")
	}
}

// TestPageUpdate_PreservesConcurrentRuntime guards the data-loss window where a
// settings PATCH arriving mid-deploy would overwrite the runtime IDs and status
// the worker has already committed, orphaning the provisioned AWS resources.
func TestPageUpdate_PreservesConcurrentRuntime(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	orgID := uuid.New()
	projectID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}

	pageID := uuid.New()
	page := &model.Page{
		BaseModel:   model.BaseModel{ID: pageID},
		ProjectID:   projectID,
		Name:        "my-page",
		Description: "old",
		GitRepo:     "https://example.com/repo.git",
		GitBranch:   "main",
		PublishPath: ".",
		Provider:    model.PageProviderAWSCloudFront,
		Region:      "us-east-1",
		Runtime:     &model.PageRuntime{},
		Status:      model.PageStatusIdle,
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// Model the real race: the worker commits runtime IDs + status to the DB row
	// *after* the PATCH handler has already loaded its snapshot. GetByID returns
	// the stale pre-deploy snapshot while the persisted row carries the fresh
	// worker state — exactly what the production Postgres store does (GetByID
	// returns a copy, not a live pointer).
	worker := &model.PageRuntime{BucketName: "prod-bucket-xyz", DistributionID: "DIST123", OACID: "OAC9"}
	must(t, fs.PagesStore.UpdateRuntime(context.Background(), pageID, worker))
	must(t, fs.PagesStore.UpdateStatus(context.Background(), pageID, model.PageStatusDeploying))
	fs.PagesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Page, error) {
		stale := *page
		stale.Runtime = &model.PageRuntime{}
		stale.Status = model.PageStatusIdle
		return &stale, nil
	}

	svc := NewPageService(fs, pages.NewRegistry(), logger, nil)

	newDesc := "new description"
	updated, err := svc.Update(context.Background(), page.ID, UpdatePageInput{Description: &newDesc})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Description != newDesc {
		t.Errorf("description = %q, want %q", updated.Description, newDesc)
	}
	if updated.Runtime == nil || updated.Runtime.BucketName != "prod-bucket-xyz" {
		t.Errorf("runtime bucket clobbered: %+v", updated.Runtime)
	}
	if updated.Runtime.DistributionID != "DIST123" || updated.Runtime.OACID != "OAC9" {
		t.Errorf("runtime IDs clobbered: %+v", updated.Runtime)
	}
	if updated.Status != model.PageStatusDeploying {
		t.Errorf("status = %q, want deploying (worker state must survive)", updated.Status)
	}

	// And the persisted row must reflect the same: no orphaning on next deploy.
	fs.PagesStore.GetByIDFn = nil // read the real stored record, not the stale stub
	got, _ := fs.PagesStore.GetByID(context.Background(), page.ID)
	if got.Runtime.BucketName != "prod-bucket-xyz" || got.Status != model.PageStatusDeploying {
		t.Errorf("persisted row clobbered: runtime=%+v status=%q", got.Runtime, got.Status)
	}
	if got.Description != newDesc {
		t.Errorf("persisted description = %q, want %q", got.Description, newDesc)
	}
}

// TestRecoverStale_DoesNotOverwriteCompletedDeploy guards the narrow window
// where the stale-recovery scan reads a deployment as "deploying" but the worker
// finishes it (success) before recovery writes. The atomic MarkTimedOut must
// leave the completed deploy — and its page — untouched.
func TestRecoverStale_DoesNotOverwriteCompletedDeploy(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewPageDeployService(fs, pages.NewRegistry(), nil, nil, nil, nil, logger)

	pageID := uuid.New()
	depID := uuid.New()
	started := time.Now().Add(-41 * time.Minute)

	// The deploy actually SUCCEEDED (worker finished it just now).
	dep := &model.PageDeployment{
		BaseModel: model.BaseModel{ID: depID, CreatedAt: started},
		PageID:    pageID,
		Status:    model.PageDeploySuccess,
		StartedAt: &started,
	}
	must(t, fs.PageDeploymentsStore.Create(context.Background(), dep))
	page := &model.Page{
		BaseModel: model.BaseModel{ID: pageID},
		Status:    model.PageStatusLive,
		Runtime:   &model.PageRuntime{},
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// The recovery scan, however, read a stale snapshot showing it still
	// deploying and older than the threshold.
	fs.PageDeploymentsStore.ListByStatusFn = func(ctx context.Context, status model.PageDeploymentStatus, params store.ListParams) ([]model.PageDeployment, int, error) {
		stale := *dep
		stale.Status = model.PageDeployDeploying
		return []model.PageDeployment{stale}, 1, nil
	}

	svc.recoverStale()

	gotDep, _ := fs.PageDeploymentsStore.GetByID(context.Background(), depID)
	if gotDep.Status != model.PageDeploySuccess {
		t.Errorf("stale recovery overwrote a completed deploy: status=%q, want success", gotDep.Status)
	}
	gotPage, _ := fs.PagesStore.GetByID(context.Background(), pageID)
	if gotPage.Status != model.PageStatusLive {
		t.Errorf("page status overwritten to %q, want live", gotPage.Status)
	}
}

// TestRecoverStale_MarksGenuinelyStaleDeploy confirms the safety net still
// works: a deploy genuinely stuck "deploying" past the threshold is failed and
// its page moved to error.
func TestRecoverStale_MarksGenuinelyStaleDeploy(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewPageDeployService(fs, pages.NewRegistry(), nil, nil, nil, nil, logger)

	pageID := uuid.New()
	depID := uuid.New()
	started := time.Now().Add(-41 * time.Minute)

	dep := &model.PageDeployment{
		BaseModel: model.BaseModel{ID: depID, CreatedAt: started},
		PageID:    pageID,
		Status:    model.PageDeployDeploying,
		StartedAt: &started,
	}
	must(t, fs.PageDeploymentsStore.Create(context.Background(), dep))
	page := &model.Page{
		BaseModel: model.BaseModel{ID: pageID},
		Status:    model.PageStatusDeploying,
		Runtime:   &model.PageRuntime{},
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	svc.recoverStale()

	gotDep, _ := fs.PageDeploymentsStore.GetByID(context.Background(), depID)
	if gotDep.Status != model.PageDeployFailed {
		t.Errorf("genuinely stale deploy not failed: status=%q", gotDep.Status)
	}
	gotPage, _ := fs.PagesStore.GetByID(context.Background(), pageID)
	if gotPage.Status != model.PageStatusError {
		t.Errorf("page status = %q, want error", gotPage.Status)
	}
}

// TestRecoverStale_PageErroredWithoutGetByID guards against the stuck-deploying
// trap: recovery must move the page to error via a write that needs no prior
// read, so a failing GetByID can't leave the page permanently "deploying" (which
// would block every future deploy via TryMarkDeploying).
func TestRecoverStale_PageErroredWithoutGetByID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewPageDeployService(fs, pages.NewRegistry(), nil, nil, nil, nil, logger)

	pageID := uuid.New()
	depID := uuid.New()
	started := time.Now().Add(-41 * time.Minute)

	dep := &model.PageDeployment{
		BaseModel: model.BaseModel{ID: depID, CreatedAt: started},
		PageID:    pageID,
		Status:    model.PageDeployDeploying,
		StartedAt: &started,
	}
	must(t, fs.PageDeploymentsStore.Create(context.Background(), dep))
	page := &model.Page{
		BaseModel: model.BaseModel{ID: pageID},
		Status:    model.PageStatusDeploying,
		Runtime:   &model.PageRuntime{},
	}
	must(t, fs.PagesStore.Create(context.Background(), page))

	// Simulate GetByID being unavailable; recovery must not depend on it.
	fs.PagesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Page, error) {
		return nil, errors.New("transient db error")
	}

	svc.recoverStale()

	fs.PagesStore.GetByIDFn = nil
	gotPage, _ := fs.PagesStore.GetByID(context.Background(), pageID)
	if gotPage.Status != model.PageStatusError {
		t.Errorf("page status = %q, want error (must not get stuck deploying)", gotPage.Status)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
