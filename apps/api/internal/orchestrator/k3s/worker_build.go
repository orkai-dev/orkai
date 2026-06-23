package k3s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

const (
	// workerBuildNamespace isolates Cloudflare Worker build pods.
	workerBuildNamespace = "orkai-worker-builds"
	// workerBuildTimeout caps a single worker build (clone + install + deploy).
	workerBuildTimeout = 20 * time.Minute
	// workerBuildIdleSleep is how long (seconds) the builder idles after a
	// successful deploy so the worker can read the wrangler output via exec
	// before the pod is deleted.
	workerBuildIdleSleep = 300
	// workerDoneMarker signals the deploy/delete script finished successfully.
	workerDoneMarker = "/out/.orkai-worker-done"
	// workerOutputFile captures the raw `wrangler` stdout/stderr for parsing.
	workerOutputFile = "/out/.wrangler-output"
	// workerCommitFile holds the deployed commit SHA (written by the clone step).
	workerCommitFile = "/out/.commit"
	// workerR2ConfirmMarker is touched by the deploy script when it stops before
	// deploying because an R2 bucket already exists and needs confirmation.
	workerR2ConfirmMarker = "/out/.orkai-needs-confirm"
	// workerR2PendingFile holds the JSON array of pending R2 buckets
	// ([{name,empty}]) written alongside workerR2ConfirmMarker.
	workerR2PendingFile = "/out/.r2-pending"
)

// BuildWorker clones a Worker's repo, installs dependencies, and runs
// `wrangler deploy` in an in-cluster build pod. The script is pushed to
// Cloudflare directly from the pod; no files are extracted. The pod receives
// the git token (to clone) and the Cloudflare API token + account id (to
// deploy), each via a short-lived Secret referenced by secretKeyRef.
func (o *Orchestrator) BuildWorker(ctx context.Context, opts orchestrator.WorkerBuildOpts) (*orchestrator.WorkerBuildResult, error) {
	return o.runWorkerPod(ctx, workerPodRequest{
		workerID:           opts.WorkerID,
		gitRepo:            opts.GitRepo,
		gitBranch:          opts.GitBranch,
		gitToken:           opts.GitToken,
		rootDirectory:      opts.RootDirectory,
		wranglerConfig:     opts.WranglerConfig,
		packageManager:     opts.PackageManager,
		installCommand:     opts.InstallCommand,
		buildCommand:       opts.BuildCommand,
		deployCommand:      opts.DeployCommand,
		buildEnvVars:       opts.BuildEnvVars,
		r2ConfirmedBuckets: opts.R2ConfirmedBuckets,
		cfAPIToken:         opts.CFAPIToken,
		cfAccountID:        opts.CFAccountID,
		script:             workerDeployScript,
		onLog:              opts.OnLog,
	})
}

// DeleteWorker clones the repo and runs `wrangler delete` to tear the Cloudflare
// script down. Best-effort: a delete failure is surfaced to the caller but does
// not block removal of the Orkai record.
func (o *Orchestrator) DeleteWorker(ctx context.Context, opts orchestrator.WorkerDeleteOpts) (*orchestrator.WorkerBuildResult, error) {
	return o.runWorkerPod(ctx, workerPodRequest{
		workerID:       opts.WorkerID,
		gitRepo:        opts.GitRepo,
		gitBranch:      opts.GitBranch,
		gitToken:       opts.GitToken,
		rootDirectory:  opts.RootDirectory,
		wranglerConfig: opts.WranglerConfig,
		packageManager: opts.PackageManager,
		installCommand: opts.InstallCommand,
		scriptName:     opts.ScriptName,
		cfAPIToken:     opts.CFAPIToken,
		cfAccountID:    opts.CFAccountID,
		script:         workerDeleteScript,
		onLog:          opts.OnLog,
	})
}

type workerPodRequest struct {
	workerID           string
	gitRepo            string
	gitBranch          string
	gitToken           string
	rootDirectory      string
	wranglerConfig     string
	packageManager     string
	installCommand     string
	buildCommand       string
	deployCommand      string
	scriptName         string
	buildEnvVars       map[string]string
	r2ConfirmedBuckets []string
	cfAPIToken         string
	cfAccountID        string
	script             string
	onLog              orchestrator.LogCallback
}

func (o *Orchestrator) runWorkerPod(ctx context.Context, req workerPodRequest) (*orchestrator.WorkerBuildResult, error) {
	start := time.Now()
	onLog := req.onLog

	if err := o.ensureNamespace(ctx, workerBuildNamespace); err != nil {
		return nil, fmt.Errorf("ensure worker build namespace: %w", err)
	}

	podName := workerBuildPodName(req.workerID)

	// Git token Secret (optional — public repos need none).
	gitSecretName := ""
	if req.gitToken != "" {
		gitSecretName = podName + "-git"
		if err := o.createWorkerSecret(ctx, gitSecretName, req.workerID, map[string]string{"token": req.gitToken}); err != nil {
			return nil, err
		}
		defer func() {
			delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			_ = o.client.CoreV1().Secrets(workerBuildNamespace).Delete(delCtx, gitSecretName, metav1.DeleteOptions{})
		}()
	}

	// Cloudflare credentials Secret (required for wrangler auth).
	cfSecretName := podName + "-cf"
	if err := o.createWorkerSecret(ctx, cfSecretName, req.workerID, map[string]string{
		"api_token":  req.cfAPIToken,
		"account_id": req.cfAccountID,
	}); err != nil {
		return nil, err
	}
	defer func() {
		delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = o.client.CoreV1().Secrets(workerBuildNamespace).Delete(delCtx, cfSecretName, metav1.DeleteOptions{})
	}()

	pod := o.buildWorkerPodSpec(podName, gitSecretName, cfSecretName, req)

	_ = o.client.CoreV1().Pods(workerBuildNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if _, err := o.client.CoreV1().Pods(workerBuildNamespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("create worker build pod: %w", err)
	}
	o.logger.Info("worker build pod created", slog.String("pod", podName), slog.String("worker_id", req.workerID))

	defer func() {
		delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = o.client.CoreV1().Pods(workerBuildNamespace).Delete(delCtx, podName, metav1.DeleteOptions{})
	}()

	var logDone chan struct{}
	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()
	if onLog != nil {
		logDone = make(chan struct{})
		go o.streamPodBuildLogs(logCtx, workerBuildNamespace, podName, []string{"git-clone", "builder"}, onLog, logDone)
	}

	if err := o.waitForWorkerBuild(ctx, podName); err != nil {
		logCancel()
		if logDone != nil {
			<-logDone
		}
		logs := o.collectPodLogs(ctx, workerBuildNamespace, podName)
		return &orchestrator.WorkerBuildResult{Logs: logs, Duration: time.Since(start)}, err
	}

	// If the deploy paused for R2 bucket confirmation, surface the pending
	// buckets to the caller instead of treating it as a successful deploy.
	if o.execInPod(ctx, workerBuildNamespace, podName, "builder", []string{"test", "-f", workerR2ConfirmMarker}, discardWriter{}) == nil {
		var pendingBuf bytes.Buffer
		_ = o.execInPod(ctx, workerBuildNamespace, podName, "builder", []string{"cat", workerR2PendingFile}, &pendingBuf)
		var commitBuf bytes.Buffer
		_ = o.execInPod(ctx, workerBuildNamespace, podName, "builder", []string{"cat", workerCommitFile}, &commitBuf)

		var pending []model.WorkerR2Bucket
		if err := json.Unmarshal(pendingBuf.Bytes(), &pending); err != nil {
			o.logger.Warn("worker build: failed to parse pending R2 buckets",
				slog.String("pod", podName), slog.Any("error", err))
		}

		logCancel()
		if logDone != nil {
			<-logDone
		}
		logs := o.collectPodLogs(ctx, workerBuildNamespace, podName)

		// The pod signalled confirmation is needed but we couldn't read any
		// pending bucket (unreadable/empty/corrupt .r2-pending). Surfacing
		// NeedsR2Confirmation with no buckets would create a confirmation the
		// user can never satisfy: confirming nothing re-triggers the deploy,
		// which re-detects the same bucket and re-pauses — an infinite loop.
		// Treat it as a build failure instead.
		if len(pending) == 0 {
			o.logger.Error("worker build: R2 confirmation requested but pending bucket list was empty or unreadable",
				slog.String("pod", podName))
			return &orchestrator.WorkerBuildResult{
				CommitSHA: strings.TrimSpace(commitBuf.String()),
				Logs:      logs,
				Duration:  time.Since(start),
			}, fmt.Errorf("R2 confirmation required but no pending buckets could be read from the build pod")
		}

		o.logger.Info("worker build paused for R2 confirmation",
			slog.String("pod", podName), slog.Int("pending_buckets", len(pending)))

		return &orchestrator.WorkerBuildResult{
			CommitSHA:           strings.TrimSpace(commitBuf.String()),
			NeedsR2Confirmation: true,
			PendingR2Buckets:    pending,
			Logs:                logs,
			Duration:            time.Since(start),
		}, nil
	}

	// Read the captured wrangler output + commit SHA (best effort).
	var outBuf bytes.Buffer
	_ = o.execInPod(ctx, workerBuildNamespace, podName, "builder", []string{"cat", workerOutputFile}, &outBuf)
	var commitBuf bytes.Buffer
	_ = o.execInPod(ctx, workerBuildNamespace, podName, "builder", []string{"cat", workerCommitFile}, &commitBuf)

	scriptName, deployedURL, deployID := parseWranglerDeployOutput(outBuf.String())
	if scriptName == "" {
		scriptName = req.scriptName
	}

	logCancel()
	if logDone != nil {
		<-logDone
	}
	logs := o.collectPodLogs(ctx, workerBuildNamespace, podName)

	o.logger.Info("worker build completed",
		slog.String("pod", podName),
		slog.String("script", scriptName),
		slog.Duration("duration", time.Since(start)),
	)

	return &orchestrator.WorkerBuildResult{
		CommitSHA:   strings.TrimSpace(commitBuf.String()),
		ScriptName:  scriptName,
		DeployedURL: deployedURL,
		DeployID:    deployID,
		Logs:        logs,
		Duration:    time.Since(start),
	}, nil
}

// createWorkerSecret stores opaque key/value data in a short-lived Secret so the
// build pod can consume it via secretKeyRef instead of an inline pod-spec value.
func (o *Orchestrator) createWorkerSecret(ctx context.Context, name, workerID string, data map[string]string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: workerBuildNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "orkai",
				"orkai/worker-build":           "true",
				"orkai/worker-id":              workerID,
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
	}
	_ = o.client.CoreV1().Secrets(workerBuildNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if _, err := o.client.CoreV1().Secrets(workerBuildNamespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create worker secret: %w", err)
	}
	return nil
}

func (o *Orchestrator) buildWorkerPodSpec(podName, gitSecretName, cfSecretName string, req workerPodRequest) *corev1.Pod {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "orkai",
		"orkai/worker-build":           "true",
		"orkai/worker-id":              req.workerID,
	}

	cloneEnv := []corev1.EnvVar{
		{Name: "GIT_REPO", Value: req.gitRepo},
		{Name: "GIT_BRANCH", Value: branchOrMain(req.gitBranch)},
	}
	var cloneScript string
	if gitSecretName != "" {
		cloneEnv = append(cloneEnv, corev1.EnvVar{
			Name: "GIT_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: gitSecretName},
					Key:                  "token",
				},
			},
		})
		cloneScript = `set -e
REPO=$(echo "$GIT_REPO" | sed "s|https://|https://x-access-token:${GIT_TOKEN}@|")
git clone --branch "$GIT_BRANCH" --depth 1 "$REPO" /workspace
git -C /workspace rev-parse HEAD > /workspace/.orkai-commit 2>/dev/null || true`
	} else {
		cloneScript = `set -e
git clone --branch "$GIT_BRANCH" --depth 1 "$GIT_REPO" /workspace
git -C /workspace rev-parse HEAD > /workspace/.orkai-commit 2>/dev/null || true`
	}

	// User-supplied build env vars first; system vars below take precedence
	// (the runtime keeps the last occurrence of a duplicate key).
	buildEnv := make([]corev1.EnvVar, 0, len(req.buildEnvVars)+12)
	for k, v := range req.buildEnvVars {
		buildEnv = append(buildEnv, corev1.EnvVar{Name: k, Value: v})
	}
	buildEnv = append(buildEnv,
		corev1.EnvVar{Name: "ORKAI_ROOT", Value: rootDirOrDot(req.rootDirectory)},
		corev1.EnvVar{Name: "ORKAI_PM", Value: pmOrAuto(req.packageManager)},
		corev1.EnvVar{Name: "ORKAI_INSTALL", Value: req.installCommand},
		corev1.EnvVar{Name: "ORKAI_BUILD", Value: req.buildCommand},
		corev1.EnvVar{Name: "ORKAI_DEPLOY", Value: req.deployCommand},
		corev1.EnvVar{Name: "ORKAI_R2_CONFIRMED", Value: strings.Join(req.r2ConfirmedBuckets, "\n")},
		corev1.EnvVar{Name: "ORKAI_WRANGLER_CONFIG", Value: wranglerConfigOrDefault(req.wranglerConfig)},
		corev1.EnvVar{Name: "ORKAI_SCRIPT_NAME", Value: strings.TrimSpace(req.scriptName)},
		corev1.EnvVar{Name: "ORKAI_IDLE_SLEEP", Value: strconv.Itoa(workerBuildIdleSleep)},
		corev1.EnvVar{Name: "CI", Value: "true"},
		corev1.EnvVar{
			Name: "CLOUDFLARE_API_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: cfSecretName},
					Key:                  "api_token",
				},
			},
		},
		corev1.EnvVar{
			Name: "CLOUDFLARE_ACCOUNT_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: cfSecretName},
					Key:                  "account_id",
				},
			},
		},
	)

	automount := false
	backoff := corev1.RestartPolicyNever

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: workerBuildNamespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                backoff,
			AutomountServiceAccountToken: &automount,
			InitContainers: []corev1.Container{
				{
					Name:    "git-clone",
					Image:   gitCloneImage,
					Command: []string{"sh", "-c", cloneScript},
					Env:     cloneEnv,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "builder",
					Image:   defaultNodeImage,
					Command: []string{"sh", "-c", req.script},
					Env:     buildEnv,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
						{Name: "out", MountPath: "/out"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "out", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}
}

// workerInstallScript is the shared install prelude (PM detection + install)
// used by both deploy and delete scripts.
const workerInstallScript = `set -e
cd "/workspace/$ORKAI_ROOT"

# Resolve the wrangler config path: keep the configured value if it exists,
# otherwise auto-detect (.jsonc / .json / .toml) so users don't have to set it.
if [ ! -f "$ORKAI_WRANGLER_CONFIG" ]; then
  for c in wrangler.jsonc wrangler.json wrangler.toml; do
    if [ -f "$c" ]; then ORKAI_WRANGLER_CONFIG="$c"; break; fi
  done
fi
export ORKAI_WRANGLER_CONFIG
echo "[orkai] wrangler config: $ORKAI_WRANGLER_CONFIG"

PM="$ORKAI_PM"
if [ "$PM" = "auto" ] || [ -z "$PM" ]; then
  if [ -f pnpm-lock.yaml ]; then PM=pnpm;
  elif [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then PM=npm;
  else PM=npm; fi
fi
echo "[orkai] package manager: $PM"

corepack enable >/dev/null 2>&1 || true
if [ "$PM" = "pnpm" ] && ! command -v pnpm >/dev/null 2>&1; then
  corepack prepare pnpm@latest --activate >/dev/null 2>&1 || npm install -g pnpm
fi

if [ -n "$ORKAI_INSTALL" ]; then
  echo "[orkai] install: $ORKAI_INSTALL"
  sh -c "$ORKAI_INSTALL"
elif [ "$PM" = "pnpm" ]; then
  if [ -f pnpm-lock.yaml ]; then pnpm install --frozen-lockfile; else pnpm install; fi
else
  if [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then npm ci; else npm install; fi
fi

mkdir -p /out
`

// workerR2BucketNamePattern matches bucket_name in wrangler.json/.jsonc (quoted key
// and value) and wrangler.toml (quoted or bare TOML strings). Word boundaries
// exclude preview_bucket_name and other *_bucket_name keys.
const workerR2BucketNamePattern = `\b"?bucket_name"?\s*[:=]\s*(?:["']([^"']+)["']|([A-Za-z0-9_-]+))`

var workerR2BucketNameRE = regexp.MustCompile(workerR2BucketNamePattern)

// workerStripConfigComments removes TOML # and JSONC // / /* */ comments so R2
// bucket extraction ignores commented-out bindings. Mirrors stripConfigComments
// in workerR2PreflightJS.
func workerStripConfigComments(text, configPath string) string {
	switch strings.ToLower(filepath.Ext(configPath)) {
	case ".toml":
		return workerStripTOMLComments(text)
	case ".json", ".jsonc":
		return workerStripJSONCComments(text)
	default:
		return text
	}
}

func workerStripTOMLComments(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && trimmed[0] == '#' {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func workerStripJSONCComments(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		if inBlock {
			if end := strings.Index(line, "*/"); end >= 0 {
				line = line[end+2:]
				inBlock = false
			} else {
				continue
			}
		}
		for {
			start := strings.Index(line, "/*")
			if start < 0 {
				break
			}
			end := strings.Index(line[start+2:], "*/")
			if end < 0 {
				line = line[:start]
				inBlock = true
				break
			}
			line = line[:start] + line[start+2+end+2:]
		}
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && strings.HasPrefix(trimmed, "//") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// workerExtractR2BucketNames returns deployment-target R2 bucket names from a
// wrangler config body (comments stripped first).
func workerExtractR2BucketNames(text, configPath string) []string {
	text = workerStripConfigComments(text, configPath)
	set := make(map[string]struct{})
	for _, m := range workerR2BucketNameRE.FindAllStringSubmatch(text, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name != "" {
			set[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	return names
}

// workerR2StripConfigCommentsJS is embedded in the build-pod preflight script.
const workerR2StripConfigCommentsJS = `
function stripConfigComments(text, file) {
  const ext = (file || "").split(".").pop().toLowerCase();
  if (ext === "toml") {
    return text.split("\n").filter((line) => {
      const t = line.trimStart();
      return !(t.length > 0 && t[0] === "#");
    }).join("\n");
  }
  if (ext === "json" || ext === "jsonc") {
    const out = [];
    let inBlock = false;
    for (const raw of text.split("\n")) {
      let line = raw;
      if (inBlock) {
        const end = line.indexOf("*/");
        if (end < 0) continue;
        line = line.slice(end + 2);
        inBlock = false;
      }
      while (true) {
        const start = line.indexOf("/*");
        if (start < 0) break;
        const end = line.indexOf("*/", start + 2);
        if (end < 0) {
          line = line.slice(0, start);
          inBlock = true;
          break;
        }
        line = line.slice(0, start) + line.slice(end + 2);
      }
      const t = line.trimStart();
      if (t.startsWith("//")) continue;
      out.push(line);
    }
    return out.join("\n");
  }
  return text;
}
`

// workerR2PreflightJS detects R2 buckets referenced by the wrangler config,
// auto-creates any that are missing, and — for buckets that already exist but
// the user hasn't approved (ORKAI_R2_CONFIRMED) — writes them to
// /out/.r2-pending and exits 3 so the deploy can pause for confirmation. All
// Cloudflare calls use the injected API token (node 22 has global fetch).
const workerR2PreflightJS = `const fs = require("fs");
` + workerR2StripConfigCommentsJS + `
function configPath() {
  const p = process.env.ORKAI_WRANGLER_CONFIG;
  if (p && fs.existsSync(p)) return p;
  for (const c of ["wrangler.jsonc", "wrangler.json", "wrangler.toml"]) {
    if (fs.existsSync(c)) return c;
  }
  return null;
}

function bucketNames(file) {
  if (!file || !fs.existsSync(file)) return [];
  const text = stripConfigComments(fs.readFileSync(file, "utf8"), file);
  const re = /` + workerR2BucketNamePattern + `/g;
  const set = new Set();
  let m;
  while ((m = re.exec(text))) set.add(m[1] || m[2]);
  return [...set];
}

const ACCOUNT = process.env.CLOUDFLARE_ACCOUNT_ID;
const TOKEN = process.env.CLOUDFLARE_API_TOKEN;
const API = "https://api.cloudflare.com/client/v4";

async function cf(method, path, name) {
  const res = await fetch(API + "/accounts/" + ACCOUNT + "/r2/buckets" + path, {
    method,
    headers: { Authorization: "Bearer " + TOKEN, "Content-Type": "application/json" },
    body: method === "POST" ? JSON.stringify({ name }) : undefined,
  });
  let json = {};
  try { json = await res.json(); } catch (e) {}
  return { ok: res.ok && json && json.success !== false, status: res.status, json };
}

async function main() {
  const names = bucketNames(configPath());
  if (names.length === 0) {
    console.log("[orkai] no R2 buckets referenced in wrangler config");
    return 0;
  }
  if (!ACCOUNT || !TOKEN) {
    console.log("[orkai] R2 preflight skipped: missing Cloudflare credentials");
    return 0;
  }
  const confirmed = new Set(
    (process.env.ORKAI_R2_CONFIRMED || "").split("\n").map((s) => s.trim()).filter(Boolean),
  );
  const pending = [];
  for (const name of names) {
    const enc = encodeURIComponent(name);
    const got = await cf("GET", "/" + enc);
    if (got.ok) {
      if (confirmed.has(name)) {
        console.log("[orkai] R2 bucket '" + name + "' already exists (confirmed) — reusing");
        continue;
      }
      let empty = false;
      try {
        const usage = await cf("GET", "/" + enc + "/usage");
        if (usage.ok) {
          const count = usage.json && usage.json.result && usage.json.result.objectCount;
          empty = !count || String(count) === "0";
        }
      } catch (e) {}
      console.log("[orkai] R2 bucket '" + name + "' already exists — needs confirmation");
      pending.push({ name, empty });
      continue;
    }
    if (got.status === 404) {
      const created = await cf("POST", "", name);
      if (created.ok) {
        console.log("[orkai] created R2 bucket '" + name + "'");
      } else if (created.status === 409 && !confirmed.has(name)) {
        // Race or stale 404: bucket exists but wasn't confirmed — pause for approval.
        console.log("[orkai] R2 bucket '" + name + "' already exists — needs confirmation");
        pending.push({ name, empty: false });
      } else if (created.status === 409) {
        console.log("[orkai] R2 bucket '" + name + "' already exists (confirmed) — reusing");
      } else {
        console.error("[orkai] failed to create R2 bucket '" + name + "' (status " + created.status + ")");
      }
      continue;
    }
    console.error("[orkai] R2 check for '" + name + "' failed (status " + got.status + ") — continuing");
  }
  if (pending.length > 0) {
    fs.writeFileSync("/out/.r2-pending", JSON.stringify(pending));
    return 3;
  }
  return 0;
}

main().then((code) => process.exit(code)).catch((err) => {
  console.error("[orkai] R2 preflight error: " + (err && err.message));
  process.exit(0);
});
`

// workerR2Preflight is the shell block that runs the preflight and pauses the
// deploy (touching the confirm + done markers) when buckets need confirmation.
const workerR2Preflight = `
cat > /tmp/orkai-r2.js <<'ORKAI_R2_EOF'
` + workerR2PreflightJS + `ORKAI_R2_EOF
echo "[orkai] checking R2 buckets…"
set +e
node /tmp/orkai-r2.js
R2STATUS=$?
set -e
if [ "$R2STATUS" -eq 3 ]; then
  echo "[orkai] deploy paused — R2 bucket(s) already exist and need confirmation"
  cp /workspace/.orkai-commit /out/.commit 2>/dev/null || true
  touch /out/.orkai-needs-confirm
  touch /out/.orkai-worker-done
  sleep "${ORKAI_IDLE_SLEEP:-300}"
  exit 0
fi
`

// workerDeployScript installs deps, runs an R2 preflight (auto-create missing
// buckets / pause on pre-existing ones), then builds and runs `wrangler
// deploy`, capturing the output for parsing and idling so the worker can read
// it via exec.
const workerDeployScript = workerInstallScript + workerR2Preflight + `
# Detect the OpenNext Cloudflare adapter so we can drive a sensible zero-config
# pipeline for vibecoded Next.js apps (build with OpenNext, deploy with wrangler,
# warm the cache best-effort).
IS_OPENNEXT=0
if [ -f package.json ] && grep -q '@opennextjs/cloudflare' package.json; then
  IS_OPENNEXT=1
fi
if [ "$PM" = "pnpm" ]; then PMX="pnpm exec"; else PMX="npx --yes"; fi

if [ "$IS_OPENNEXT" = "1" ]; then
  # Canonical OpenNext pipeline. We deliberately do NOT use opennextjs-cloudflare
  # deploy: its remote R2 incremental-cache population calls the Cloudflare API
  # through the bundled cloudflare SDK (node-fetch), which deterministically
  # fails with "Premature close" from in-cluster build pods even though wrangler
  # deploy and plain fetch to the same API succeed. Instead: OpenNext build ->
  # wrangler deploy (uploads the worker) -> warm the R2 cache best-effort (a
  # cache-warm hiccup must never undo a live deploy; the cache warms on demand at
  # runtime if it is skipped).
  echo "[orkai] OpenNext detected — build + wrangler deploy (cache warmed best-effort)"
  set +e
  $PMX opennextjs-cloudflare build
  BSTATUS=$?
  set -e
  if [ "$BSTATUS" -ne 0 ]; then
    echo "[orkai] build failed (exit $BSTATUS)" >&2
    exit "$BSTATUS"
  fi

  echo "[orkai] wrangler deploy (config: $ORKAI_WRANGLER_CONFIG)"
  set +e
  npx --yes wrangler deploy -c "$ORKAI_WRANGLER_CONFIG" > /out/.wrangler-output 2>&1
  WSTATUS=$?
  set -e
  cat /out/.wrangler-output
  if [ "$WSTATUS" -ne 0 ]; then
    echo "[orkai] deploy failed (exit $WSTATUS)" >&2
    exit "$WSTATUS"
  fi

  # OpenNext's remote cache population checks the R2 bucket via the cloudflare
  # SDK, which uses node-fetch + agentkeepalive and premature-closes against the
  # Cloudflare API from in-cluster build pods. The runtime's native fetch
  # (undici) works against the same endpoint, so inject it into the SDK's
  # instances before populating. Patch is in the ephemeral pod only.
  ORKAI_ON_DIR=$(ls -d node_modules/.pnpm/@opennextjs+cloudflare@*/node_modules/@opennextjs/cloudflare 2>/dev/null | head -1)
  ORKAI_ENSURE="$ORKAI_ON_DIR/dist/cli/utils/ensure-r2-bucket.js"
  if [ -n "$ORKAI_ON_DIR" ] && [ -f "$ORKAI_ENSURE" ]; then
    node -e "const fs=require('fs');const f=process.argv[1];let s=fs.readFileSync(f,'utf8');if(!s.includes('fetch: globalThis.fetch')){fs.writeFileSync(f,s.split('new Cloudflare({').join('new Cloudflare({ fetch: globalThis.fetch,'));console.log('[orkai] patched OpenNext R2 bucket check to use native fetch (undici)');}else{console.log('[orkai] OpenNext R2 bucket check already patched');}" "$ORKAI_ENSURE" || echo "[orkai] warning: could not patch OpenNext R2 bucket check" >&2
  fi
  echo "[orkai] populating OpenNext R2 incremental cache (best-effort)…"
  set +e
  $PMX opennextjs-cloudflare populateCache remote
  PSTATUS=$?
  set -e
  if [ "$PSTATUS" -ne 0 ]; then
    echo "[orkai] warning: R2 cache population failed (exit $PSTATUS) — worker is live; ISR cache will warm on demand" >&2
  fi
else
  if [ -n "$ORKAI_BUILD" ]; then
    echo "[orkai] build: $ORKAI_BUILD"
    set +e
    sh -c "$ORKAI_BUILD"
    BSTATUS=$?
    set -e
    if [ "$BSTATUS" -ne 0 ]; then
      echo "[orkai] build failed (exit $BSTATUS)" >&2
      exit "$BSTATUS"
    fi
  fi

  if [ -n "$ORKAI_DEPLOY" ]; then
    echo "[orkai] deploy: $ORKAI_DEPLOY"
    DEPLOY_CMD="$ORKAI_DEPLOY"
  else
    echo "[orkai] wrangler deploy (config: $ORKAI_WRANGLER_CONFIG)"
    DEPLOY_CMD="npx --yes wrangler deploy -c \"$ORKAI_WRANGLER_CONFIG\""
  fi
  set +e
  sh -c "$DEPLOY_CMD" > /out/.wrangler-output 2>&1
  WSTATUS=$?
  set -e
  cat /out/.wrangler-output
  if [ "$WSTATUS" -ne 0 ]; then
    echo "[orkai] deploy failed (exit $WSTATUS)" >&2
    exit "$WSTATUS"
  fi
fi

cp /workspace/.orkai-commit /out/.commit 2>/dev/null || true
touch /out/.orkai-worker-done
echo "[orkai] deploy complete; ready for extraction"
sleep "${ORKAI_IDLE_SLEEP:-300}"
`

// workerDeleteScript installs deps then runs `wrangler delete` (best-effort).
const workerDeleteScript = workerInstallScript + `
echo "[orkai] wrangler delete (config: $ORKAI_WRANGLER_CONFIG)"
set +e
if [ -n "$ORKAI_SCRIPT_NAME" ]; then
  npx wrangler delete -c "$ORKAI_WRANGLER_CONFIG" --name "$ORKAI_SCRIPT_NAME" > /out/.wrangler-output 2>&1
else
  npx wrangler delete -c "$ORKAI_WRANGLER_CONFIG" > /out/.wrangler-output 2>&1
fi
WSTATUS=$?
set -e
cat /out/.wrangler-output
if [ "$WSTATUS" -ne 0 ]; then
  echo "[orkai] wrangler delete failed (exit $WSTATUS)" >&2
  exit "$WSTATUS"
fi
cp /workspace/.orkai-commit /out/.commit 2>/dev/null || true
touch /out/.orkai-worker-done
echo "[orkai] delete complete"
sleep "${ORKAI_IDLE_SLEEP:-300}"
`

// waitForWorkerBuild polls the build pod until the success marker appears or the
// build fails / times out.
func (o *Orchestrator) waitForWorkerBuild(ctx context.Context, podName string) error {
	deadline := time.Now().Add(workerBuildTimeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("worker build timed out after %s", workerBuildTimeout)
		}

		pod, err := o.client.CoreV1().Pods(workerBuildNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		if pod.Status.Phase == corev1.PodFailed {
			return fmt.Errorf("worker build failed: %s", buildFailureReason(pod))
		}

		builderRunning := false
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != "builder" {
				continue
			}
			if cs.State.Terminated != nil {
				if cs.State.Terminated.ExitCode != 0 {
					return fmt.Errorf("worker build failed: builder exited with code %d", cs.State.Terminated.ExitCode)
				}
				return fmt.Errorf("builder exited before output could be read; ensure ORKAI_IDLE_SLEEP is not overridden and the deploy completes within %s", workerBuildTimeout)
			}
			if cs.State.Running != nil {
				builderRunning = true
			}
		}

		if builderRunning {
			if o.execInPod(ctx, workerBuildNamespace, podName, "builder", []string{"test", "-f", workerDoneMarker}, discardWriter{}) == nil {
				return nil
			}
		}

		time.Sleep(3 * time.Second)
	}
}

// discardWriter is an io.Writer that drops all writes (avoids importing io for a
// single Discard reference in this file).
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// parseWranglerDeployOutput best-effort extracts the script name, deployed URL,
// and deployment/version ID from `wrangler deploy` output.
func parseWranglerDeployOutput(out string) (scriptName, deployedURL, deployID string) {
	for _, raw := range strings.Split(out, "\n") {
		l := strings.TrimSpace(raw)

		if deployedURL == "" {
			if i := strings.Index(l, "https://"); i >= 0 {
				cand := strings.Fields(l[i:])
				if len(cand) > 0 && strings.Contains(cand[0], ".workers.dev") {
					deployedURL = strings.TrimRight(cand[0], ".,)")
				}
			}
		}

		if deployID == "" {
			for _, prefix := range []string{"Current Deployment ID:", "Current Version ID:", "Deployment ID:", "Version ID:"} {
				if idx := strings.Index(l, prefix); idx >= 0 {
					deployID = strings.TrimSpace(l[idx+len(prefix):])
					break
				}
			}
		}

		if scriptName == "" {
			// "Deployed <name> triggers (...)" and "Published <name> (...)" name
			// the script authoritatively. "Uploaded <name> (...)" does too, but
			// wrangler also emits asset lines like "Uploaded 1 of 1 asset" — which
			// must NOT be mistaken for the script name (the parenthesised duration
			// only appears on the script line, so require it for "Uploaded ").
			switch {
			case strings.HasPrefix(l, "Deployed "):
				if fields := strings.Fields(strings.TrimPrefix(l, "Deployed ")); len(fields) > 0 {
					scriptName = fields[0]
				}
			case strings.HasPrefix(l, "Published "):
				if fields := strings.Fields(strings.TrimPrefix(l, "Published ")); len(fields) > 0 {
					scriptName = fields[0]
				}
			case strings.HasPrefix(l, "Uploaded ") && strings.Contains(l, "("):
				if fields := strings.Fields(strings.TrimPrefix(l, "Uploaded ")); len(fields) > 0 {
					scriptName = fields[0]
				}
			}
		}
	}

	// Derive the script name from the URL subdomain when not stated explicitly.
	if scriptName == "" && deployedURL != "" {
		host := strings.TrimPrefix(strings.TrimPrefix(deployedURL, "https://"), "http://")
		if i := strings.Index(host, "."); i > 0 {
			scriptName = host[:i]
		}
	}
	return scriptName, deployedURL, deployID
}

func workerBuildPodName(workerID string) string {
	id := strings.ReplaceAll(workerID, "-", "")
	if len(id) > 12 {
		id = id[:12]
	}
	if id == "" {
		id = "worker"
	}
	name := fmt.Sprintf("worker-build-%s-%d", id, time.Now().Unix())
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func wranglerConfigOrDefault(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "wrangler.toml"
	}
	return c
}
