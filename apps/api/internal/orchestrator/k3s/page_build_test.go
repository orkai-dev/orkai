package k3s

import (
	"archive/tar"
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

func testOrchestrator() *Orchestrator {
	return &Orchestrator{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func envValue(env []corev1.EnvVar, name string) (string, bool) {
	for _, e := range env {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

func findEnv(env []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range env {
		if env[i].Name == name {
			return &env[i]
		}
	}
	return nil
}

func TestPageBuildPodName(t *testing.T) {
	name := pageBuildPodName("11111111-2222-3333-4444-555555555555")
	if !strings.HasPrefix(name, "page-build-") {
		t.Errorf("name %q should start with page-build-", name)
	}
	if len(name) > 63 {
		t.Errorf("name %q exceeds 63 chars", name)
	}
	// Empty page id still yields a valid name.
	if n := pageBuildPodName(""); !strings.HasPrefix(n, "page-build-") {
		t.Errorf("empty page id name %q invalid", n)
	}
}

func TestBuildStaticPodSpec(t *testing.T) {
	o := testOrchestrator()
	pod := o.buildStaticPodSpec("page-build-abc-1", "page-build-abc-1-git", orchestrator.StaticBuildOpts{
		GitRepo:        "https://github.com/acme/site",
		GitBranch:      "release",
		GitToken:       "secret-token",
		PageID:         "page-1",
		RootDirectory:  "apps/web",
		PackageManager: "pnpm",
		OutputDir:      "build",
		BuildEnvVars:   map[string]string{"VITE_API_URL": "https://api.example.com"},
	})

	if pod.Namespace != pageBuildNamespace {
		t.Errorf("namespace = %q, want %q", pod.Namespace, pageBuildNamespace)
	}
	if len(pod.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(pod.Spec.InitContainers))
	}
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
	}

	// Clone container receives repo/branch via env; the token comes from a
	// Secret via secretKeyRef (never inlined as a plaintext pod-spec value).
	clone := pod.Spec.InitContainers[0]
	if v, _ := envValue(clone.Env, "GIT_BRANCH"); v != "release" {
		t.Errorf("GIT_BRANCH = %q, want release", v)
	}
	if clone.Image != gitCloneImage {
		t.Errorf("git-clone image = %q, want pinned %q", clone.Image, gitCloneImage)
	}
	tokenEnv := findEnv(clone.Env, "GIT_TOKEN")
	if tokenEnv == nil {
		t.Fatal("GIT_TOKEN env missing")
	}
	if tokenEnv.Value != "" {
		t.Errorf("GIT_TOKEN must not be inlined as a plaintext value, got %q", tokenEnv.Value)
	}
	if tokenEnv.ValueFrom == nil || tokenEnv.ValueFrom.SecretKeyRef == nil {
		t.Fatal("GIT_TOKEN must be sourced from a Secret via secretKeyRef")
	}
	if tokenEnv.ValueFrom.SecretKeyRef.Name != "page-build-abc-1-git" || tokenEnv.ValueFrom.SecretKeyRef.Key != "token" {
		t.Errorf("GIT_TOKEN secretKeyRef = %+v, want secret page-build-abc-1-git key token", tokenEnv.ValueFrom.SecretKeyRef)
	}

	// Builder container receives build config + custom env.
	builder := pod.Spec.Containers[0]
	if v, _ := envValue(builder.Env, "ORKAI_OUTPUT"); v != "build" {
		t.Errorf("ORKAI_OUTPUT = %q, want build", v)
	}
	if v, _ := envValue(builder.Env, "ORKAI_PM"); v != "pnpm" {
		t.Errorf("ORKAI_PM = %q, want pnpm", v)
	}
	if v, _ := envValue(builder.Env, "ORKAI_ROOT"); v != "apps/web" {
		t.Errorf("ORKAI_ROOT = %q, want apps/web", v)
	}
	if v, _ := envValue(builder.Env, "VITE_API_URL"); v != "https://api.example.com" {
		t.Errorf("custom build env VITE_API_URL = %q", v)
	}
	// Idle-sleep is injected from the constant so the script can't drift from it.
	if v, _ := envValue(builder.Env, "ORKAI_IDLE_SLEEP"); v != strconv.Itoa(pageBuildIdleSleep) {
		t.Errorf("ORKAI_IDLE_SLEEP = %q, want %d", v, pageBuildIdleSleep)
	}
	if builder.Image != defaultNodeImage {
		t.Errorf("image = %q, want default %q", builder.Image, defaultNodeImage)
	}

	// Two volumes: workspace (shared) and out (output staging).
	names := map[string]bool{}
	for _, v := range pod.Spec.Volumes {
		names[v.Name] = true
	}
	if !names["workspace"] || !names["out"] {
		t.Errorf("expected workspace + out volumes, got %v", names)
	}
}

func TestBuildStaticPodSpec_NodeVersionAndDefaults(t *testing.T) {
	o := testOrchestrator()
	pod := o.buildStaticPodSpec("p", "", orchestrator.StaticBuildOpts{
		GitRepo:   "https://github.com/acme/site",
		NodeImage: "node:20-bookworm-slim",
		// no token, no root, no pm
	})
	clone := pod.Spec.InitContainers[0]
	if _, ok := envValue(clone.Env, "GIT_TOKEN"); ok {
		t.Error("GIT_TOKEN should be absent for public repos")
	}
	if v, _ := envValue(clone.Env, "GIT_BRANCH"); v != "main" {
		t.Errorf("default branch = %q, want main", v)
	}
	builder := pod.Spec.Containers[0]
	if v, _ := envValue(builder.Env, "ORKAI_ROOT"); v != "." {
		t.Errorf("default root = %q, want .", v)
	}
	if v, _ := envValue(builder.Env, "ORKAI_PM"); v != "auto" {
		t.Errorf("default pm = %q, want auto", v)
	}
	if builder.Image != "node:20-bookworm-slim" {
		t.Errorf("image = %q, want override", builder.Image)
	}
}

func TestUntarToDir(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	files := map[string]string{
		"index.html":     "<h1>hi</h1>",
		"assets/app.js":  "console.log(1)",
		"nested/a/b.txt": "deep",
	}
	for name, body := range files {
		must := func(err error) {
			if err != nil {
				t.Fatal(err)
			}
		}
		must(tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(body))
		must(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := untarToDir(&buf, dest); err != nil {
		t.Fatalf("untarToDir: %v", err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestUntarToDir_RejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	body := "evil"
	_ = tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte(body))
	_ = tw.Close()

	dest := t.TempDir()
	if err := untarToDir(&buf, dest); err == nil {
		t.Error("expected path-traversal tar entry to be rejected")
	}
}
