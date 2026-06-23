package k3s

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

const (
	// pageBuildNamespace isolates static-site build pods from app workloads.
	pageBuildNamespace = "orkai-page-builds"
	// defaultNodeImage is the build image when a page does not pin a version.
	defaultNodeImage = "node:22-bookworm-slim"
	// gitCloneImage is pinned (not :latest) so the clone step is reproducible
	// and a registry push can't silently change the binary that pulls source.
	gitCloneImage = "alpine/git:2.45.2"
	// pageBuildTimeout caps a single static build (clone + install + build).
	pageBuildTimeout = 30 * time.Minute
	// pageBuildIdleSleep is how long (seconds) the builder idles after a
	// successful build so the worker can extract the output via exec before the
	// pod is deleted. Injected into the build script as ORKAI_IDLE_SLEEP so the
	// script and this value can never drift apart.
	pageBuildIdleSleep = 1800
	// buildDoneMarker signals the build script finished successfully.
	buildDoneMarker = "/out/.orkai-build-done"
	// buildCommitFile holds the built commit SHA (written by the clone step).
	buildCommitFile = "/out/.commit"
)

// BuildStatic clones a page's repo, runs install + build (npm/pnpm) in an
// in-cluster build pod, and extracts the build output to a local directory the
// caller can sync to object storage. The build pod never receives AWS
// credentials; only the git token (to clone) and the build env vars enter it.
func (o *Orchestrator) BuildStatic(ctx context.Context, opts orchestrator.StaticBuildOpts) (*orchestrator.StaticBuildResult, error) {
	start := time.Now()
	onLog := opts.OnLog
	logf := func(format string, args ...any) {
		if onLog != nil {
			onLog(fmt.Sprintf(format, args...) + "\n")
		}
	}

	if err := o.ensureNamespace(ctx, pageBuildNamespace); err != nil {
		return nil, fmt.Errorf("ensure build namespace: %w", err)
	}

	podName := pageBuildPodName(opts.PageID)

	// Carry the git token in a short-lived Secret (referenced via secretKeyRef)
	// rather than inlining it in the pod spec, so it never shows up in
	// `kubectl get pod -o yaml` or the API-server audit log. Deleted on the way
	// out (detached context so cleanup runs even if ctx is cancelled).
	gitSecretName := ""
	if opts.GitToken != "" {
		gitSecretName = podName + "-git"
		if err := o.createGitSecret(ctx, gitSecretName, opts); err != nil {
			return nil, err
		}
		defer func() {
			delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			_ = o.client.CoreV1().Secrets(pageBuildNamespace).Delete(delCtx, gitSecretName, metav1.DeleteOptions{})
		}()
	}

	pod := o.buildStaticPodSpec(podName, gitSecretName, opts)

	// Remove any leftover pod with the same name, then create.
	_ = o.client.CoreV1().Pods(pageBuildNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if _, err := o.client.CoreV1().Pods(pageBuildNamespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("create build pod: %w", err)
	}
	o.logger.Info("page build pod created", slog.String("pod", podName), slog.String("page_id", opts.PageID))

	// Always clean up the pod, even on cancellation, using a detached context.
	defer func() {
		delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = o.client.CoreV1().Pods(pageBuildNamespace).Delete(delCtx, podName, metav1.DeleteOptions{})
	}()

	// Stream init + main container logs into the deploy log.
	var logDone chan struct{}
	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()
	if onLog != nil {
		logDone = make(chan struct{})
		go o.streamPodBuildLogs(logCtx, pageBuildNamespace, podName, []string{"git-clone", "builder"}, onLog, logDone)
	}

	// Wait for the build to succeed (marker present) or fail (pod/container error).
	if err := o.waitForStaticBuild(ctx, podName); err != nil {
		logCancel()
		if logDone != nil {
			<-logDone
		}
		logs := o.collectPodLogs(ctx, pageBuildNamespace, podName)
		return &orchestrator.StaticBuildResult{Logs: logs, Duration: time.Since(start)}, err
	}

	logf("[orkai] extracting build output…")

	// Read the built commit SHA (best effort).
	var commitBuf bytes.Buffer
	_ = o.execInPod(ctx, pageBuildNamespace, podName, "builder", []string{"cat", buildCommitFile}, &commitBuf)
	commitSHA := strings.TrimSpace(commitBuf.String())

	// Extract the output directory via `tar` over exec into a fresh temp dir.
	destDir, err := os.MkdirTemp("", "orkai-page-build-*")
	if err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(destDir) }
	if err := o.extractBuildOutput(ctx, podName, destDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("extract build output: %w", err)
	}

	logCancel()
	if logDone != nil {
		<-logDone
	}
	logs := o.collectPodLogs(ctx, pageBuildNamespace, podName)

	o.logger.Info("page build completed",
		slog.String("pod", podName),
		slog.String("commit", commitSHA),
		slog.Duration("duration", time.Since(start)),
	)

	return &orchestrator.StaticBuildResult{
		FilesDir:  destDir,
		CommitSHA: commitSHA,
		Cleanup:   cleanup,
		Logs:      logs,
		Duration:  time.Since(start),
	}, nil
}

// createGitSecret stores the git token in a short-lived Opaque Secret so the
// build pod can consume it via secretKeyRef instead of an inline pod-spec value.
func (o *Orchestrator) createGitSecret(ctx context.Context, name string, opts orchestrator.StaticBuildOpts) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pageBuildNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "orkai",
				"orkai/page-build":             "true",
				"orkai/page-id":                opts.PageID,
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"token": opts.GitToken},
	}
	_ = o.client.CoreV1().Secrets(pageBuildNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if _, err := o.client.CoreV1().Secrets(pageBuildNamespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create git secret: %w", err)
	}
	return nil
}

// buildStaticPodSpec assembles the build pod: an init container clones the repo,
// the main node container detects the package manager, installs, builds, copies
// the output to /out, then idles so the worker can extract it. When
// gitSecretName is set, the clone step reads the token from that Secret via
// secretKeyRef rather than from an inline pod-spec env value.
func (o *Orchestrator) buildStaticPodSpec(podName, gitSecretName string, opts orchestrator.StaticBuildOpts) *corev1.Pod {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "orkai",
		"orkai/page-build":             "true",
		"orkai/page-id":                opts.PageID,
	}

	nodeImage := opts.NodeImage
	if nodeImage == "" {
		nodeImage = defaultNodeImage
	}

	// Clone in an init container (alpine/git has git); record the commit SHA so
	// the build container — which may not have git — can surface it.
	cloneEnv := []corev1.EnvVar{
		{Name: "GIT_REPO", Value: opts.GitRepo},
		{Name: "GIT_BRANCH", Value: branchOrMain(opts.GitBranch)},
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

	// Build env vars are injected natively (no shell quoting concerns).
	// User-supplied vars are prepended so the system vars below always take
	// precedence (the container runtime keeps the last occurrence of a
	// duplicate key); this prevents a page's build_env_vars from overriding
	// ORKAI_IDLE_SLEEP, ORKAI_OUTPUT, CI, etc.
	buildEnv := make([]corev1.EnvVar, 0, len(opts.BuildEnvVars)+7)
	for k, v := range opts.BuildEnvVars {
		buildEnv = append(buildEnv, corev1.EnvVar{Name: k, Value: v})
	}
	buildEnv = append(buildEnv,
		corev1.EnvVar{Name: "ORKAI_ROOT", Value: rootDirOrDot(opts.RootDirectory)},
		corev1.EnvVar{Name: "ORKAI_PM", Value: pmOrAuto(opts.PackageManager)},
		corev1.EnvVar{Name: "ORKAI_INSTALL", Value: opts.InstallCommand},
		corev1.EnvVar{Name: "ORKAI_BUILD", Value: opts.BuildCommand},
		corev1.EnvVar{Name: "ORKAI_OUTPUT", Value: strings.TrimSpace(opts.OutputDir)},
		// Keep the idle sleep in sync with pageBuildIdleSleep (single source of truth).
		corev1.EnvVar{Name: "ORKAI_IDLE_SLEEP", Value: strconv.Itoa(pageBuildIdleSleep)},
		corev1.EnvVar{Name: "CI", Value: "true"},
	)

	automount := false
	backoff := corev1.RestartPolicyNever

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pageBuildNamespace,
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
					Image:   nodeImage,
					Command: []string{"sh", "-c", staticBuildScript},
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

// staticBuildScript runs in the builder container. It infers the package
// manager from the lockfile (when ORKAI_PM=auto), installs, builds, copies the
// output to /out, then idles so the worker can extract the files.
const staticBuildScript = `set -e
cd "/workspace/$ORKAI_ROOT"

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

if [ -n "$ORKAI_BUILD" ]; then
  echo "[orkai] build: $ORKAI_BUILD"
  sh -c "$ORKAI_BUILD"
else
  echo "[orkai] build: $PM run build"
  "$PM" run build
fi

if [ -z "$ORKAI_OUTPUT" ]; then
  echo "[orkai] ERROR: output directory is not set" >&2
  exit 2
fi
if [ ! -d "$ORKAI_OUTPUT" ]; then
  echo "[orkai] ERROR: output directory '$ORKAI_OUTPUT' not found after build" >&2
  exit 2
fi

mkdir -p /out
cp -a "$ORKAI_OUTPUT/." /out/
cp /workspace/.orkai-commit /out/.commit 2>/dev/null || true
touch /out/.orkai-build-done
echo "[orkai] build complete; output ready for extraction"
sleep "${ORKAI_IDLE_SLEEP:-1800}"
`

// waitForStaticBuild polls the build pod until the success marker appears or the
// build fails (pod/container error) or times out.
func (o *Orchestrator) waitForStaticBuild(ctx context.Context, podName string) error {
	deadline := time.Now().Add(pageBuildTimeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("build timed out after %s", pageBuildTimeout)
		}

		pod, err := o.client.CoreV1().Pods(pageBuildNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		if pod.Status.Phase == corev1.PodFailed {
			return fmt.Errorf("build failed: %s", buildFailureReason(pod))
		}

		builderRunning := false
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != "builder" {
				continue
			}
			if cs.State.Terminated != nil {
				if cs.State.Terminated.ExitCode != 0 {
					return fmt.Errorf("build failed: builder exited with code %d", cs.State.Terminated.ExitCode)
				}
				// Builder exited cleanly before the output could be extracted
				// (the idle-sleep window elapsed). Exec into a terminated
				// container is impossible, so surface a clear error rather than
				// silently polling until the timeout.
				return fmt.Errorf("builder exited before output could be extracted; ensure ORKAI_IDLE_SLEEP is not overridden and the build completes within %s", pageBuildTimeout)
			}
			if cs.State.Running != nil {
				builderRunning = true
			}
		}

		if builderRunning {
			// Marker present => install + build succeeded and output is staged.
			if o.execInPod(ctx, pageBuildNamespace, podName, "builder", []string{"test", "-f", buildDoneMarker}, io.Discard) == nil {
				return nil
			}
		}

		time.Sleep(3 * time.Second)
	}
}

// extractBuildOutput streams a tar of /out (minus orkai marker files) out of the
// pod and unpacks it into destDir.
func (o *Orchestrator) extractBuildOutput(ctx context.Context, podName, destDir string) error {
	pr, pw := io.Pipe()
	go func() {
		err := o.execInPod(ctx, pageBuildNamespace, podName, "builder",
			[]string{"tar", "cf", "-", "-C", "/out", "--exclude=./.orkai-build-done", "--exclude=./.commit", "."}, pw)
		_ = pw.CloseWithError(err)
	}()
	return untarToDir(pr, destDir)
}

// execInPod runs a command in a pod container and writes stdout to out. It
// returns an error (including the captured stderr) when the command fails.
func (o *Orchestrator) execInPod(ctx context.Context, namespace, podName, container string, cmd []string, out io.Writer) error {
	req := o.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(o.config, "POST", req.URL())
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: out,
		Stderr: &stderr,
	}); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// streamPageBuildLogs follows the given containers' logs in order and pushes
// them through onLog. Mirrors the app build log streamer but addresses the pod
// by name (no Job wrapper).
func (o *Orchestrator) streamPodBuildLogs(ctx context.Context, namespace, podName string, containers []string, onLog orchestrator.LogCallback, done chan struct{}) {
	defer close(done)

	var buf strings.Builder
	lastFlush := time.Now()
	flush := func() {
		if buf.Len() > 0 {
			onLog(buf.String())
			buf.Reset()
			lastFlush = time.Now()
		}
	}

	for _, c := range containers {
		if ctx.Err() != nil {
			break
		}
		fmt.Fprintf(&buf, "=== %s ===\n", c)
		flush()

		if !o.waitForContainerRunning(ctx, namespace, podName, c) {
			continue
		}

		logStream, err := o.client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: c,
			Follow:    true,
		}).Stream(ctx)
		if err != nil {
			fmt.Fprintf(&buf, "[failed to stream %s: %s]\n", c, err)
			flush()
			continue
		}

		scanner := bufio.NewScanner(logStream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			buf.WriteString(scanner.Text() + "\n")
			if time.Since(lastFlush) > 3*time.Second {
				flush()
			}
		}
		_ = logStream.Close()
		flush()
	}
}

// collectPodLogs gathers the full logs from all containers of the build pod.
func (o *Orchestrator) collectPodLogs(ctx context.Context, namespace, podName string) string {
	pod, err := o.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return ""
	}
	var all strings.Builder
	for _, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		stream, err := o.client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: c.Name,
		}).Stream(ctx)
		if err != nil {
			continue
		}
		fmt.Fprintf(&all, "=== %s ===\n", c.Name)
		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			all.WriteString(scanner.Text() + "\n")
		}
		_ = stream.Close()
	}
	return all.String()
}

// buildFailureReason extracts a human-readable reason from a failed build pod.
func buildFailureReason(pod *corev1.Pod) string {
	for _, cs := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			msg := cs.State.Terminated.Reason
			if cs.State.Terminated.Message != "" {
				msg = cs.State.Terminated.Message
			}
			return fmt.Sprintf("%s exited with code %d (%s)", cs.Name, cs.State.Terminated.ExitCode, msg)
		}
	}
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	return string(pod.Status.Phase)
}

// untarToDir unpacks a tar stream into dest, rejecting path traversal and
// skipping symlinks/hardlinks for safety.
func untarToDir(r io.Reader, dest string) error {
	cleanDest := filepath.Clean(dest)
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(cleanDest, hdr.Name)
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar path %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // size bounded by build output
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// Skip symlinks, hardlinks, devices, etc.
		}
	}
}

func pageBuildPodName(pageID string) string {
	id := strings.ReplaceAll(pageID, "-", "")
	if len(id) > 12 {
		id = id[:12]
	}
	if id == "" {
		id = "page"
	}
	name := fmt.Sprintf("page-build-%s-%d", id, time.Now().Unix())
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func branchOrMain(b string) string {
	if strings.TrimSpace(b) == "" {
		return "main"
	}
	return b
}

func rootDirOrDot(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return "."
	}
	return d
}

func pmOrAuto(pm string) string {
	pm = strings.TrimSpace(pm)
	if pm == "" {
		return "auto"
	}
	return pm
}
