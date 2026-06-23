package k3s

import (
	"sort"
	"strings"
	"testing"
)

func r2BucketNamesMatch(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("matches = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("match[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestWorkerR2BucketNamePattern(t *testing.T) {

	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "json double quoted",
			input: `"bucket_name": "my-project-opennext-cache-v2"`,
			want:  []string{"my-project-opennext-cache-v2"},
		},
		{
			name:  "toml single quoted",
			input: `bucket_name = 'legacy-cache'`,
			want:  []string{"legacy-cache"},
		},
		{
			name:  "toml unquoted",
			input: `bucket_name = my-project-opennext-cache-v2`,
			want:  []string{"my-project-opennext-cache-v2"},
		},
		{
			name: "multiple bindings",
			input: `
[[r2_buckets]]
bucket_name = cache-a

[[r2_buckets]]
binding = "OTHER"
bucket_name: "cache-b"
`,
			want: []string{"cache-a", "cache-b"},
		},
		{
			name: "ignores preview_bucket_name",
			input: `
[[r2_buckets]]
bucket_name = "main-cache"
preview_bucket_name = "preview-cache"
`,
			want: []string{"main-cache"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := workerExtractR2BucketNames(tc.input, "wrangler.toml")
			r2BucketNamesMatch(t, got, tc.want)
		})
	}
}

func TestWorkerExtractR2BucketNames_IgnoresComments(t *testing.T) {
	cases := []struct {
		name       string
		configPath string
		input      string
		want       []string
	}{
		{
			name:       "toml commented binding",
			configPath: "wrangler.toml",
			input: `
[[r2_buckets]]
binding = "CACHE"
bucket_name = "live-cache"

# [[r2_buckets]]
# bucket_name = "ghost-cache"
`,
			want: []string{"live-cache"},
		},
		{
			name:       "jsonc line comment",
			configPath: "wrangler.jsonc",
			input: `{
  "r2_buckets": [{ "bucket_name": "live-cache" }],
  // "bucket_name": "ghost-cache"
}`,
			want: []string{"live-cache"},
		},
		{
			name:       "jsonc block comment",
			configPath: "wrangler.jsonc",
			input: `{
  "r2_buckets": [{ "bucket_name": "live-cache" }],
  /* "bucket_name": "ghost-cache" */
}`,
			want: []string{"live-cache"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := workerExtractR2BucketNames(tc.input, tc.configPath)
			r2BucketNamesMatch(t, got, tc.want)
		})
	}
}

func TestWorkerR2PreflightJS_StripComments(t *testing.T) {
	if !strings.Contains(workerR2PreflightJS, "stripConfigComments(") {
		t.Fatal("preflight must strip config comments before extracting bucket names")
	}
}

func TestWorkerR2PreflightJS_OnlyCreatesOn404(t *testing.T) {
	if strings.Contains(workerR2PreflightJS, "got.json.success === false") {
		t.Fatal("preflight must not treat Cloudflare success:false as bucket-not-found")
	}
	if !strings.Contains(workerR2PreflightJS, "got.status === 404") {
		t.Fatal("preflight must gate bucket creation on HTTP 404 only")
	}
}

func TestParseWranglerDeployOutput(t *testing.T) {
	// Real OpenNext + wrangler deploy output: the asset-count line ("Uploaded 1
	// of 1 asset") precedes the script line and must not be mistaken for the
	// script name (previously yielded "1", breaking teardown).
	out := `🌀 Building list of assets...
✨ Read 45 files from the assets directory /workspace/.open-next/assets
🌀 Starting asset upload...
🌀 Found 1 new or modified static asset to upload. Proceeding with upload...
+ /BUILD_ID
Uploaded 1 of 1 asset
✨ Success! Uploaded 1 file (38 already uploaded) (1.51 sec)

Total Upload: 4833.07 KiB / gzip: 1007.63 KiB
Worker Startup Time: 30 ms
Uploaded poker-planning-app (12.47 sec)
Deployed poker-planning-app triggers (2.96 sec)
  https://poker-planning-app.germaneichemberger.workers.dev
Current Version ID: 7a68e8e9-e897-4a1e-8edb-452bcbc0fd4b`

	script, url, id := parseWranglerDeployOutput(out)
	if script != "poker-planning-app" {
		t.Fatalf("scriptName = %q, want %q", script, "poker-planning-app")
	}
	if url != "https://poker-planning-app.germaneichemberger.workers.dev" {
		t.Fatalf("deployedURL = %q", url)
	}
	if id != "7a68e8e9-e897-4a1e-8edb-452bcbc0fd4b" {
		t.Fatalf("deployID = %q", id)
	}
}

func TestParseWranglerDeployOutputDeployedOnly(t *testing.T) {
	// No "Uploaded <name> (...)" line — script name comes from "Deployed".
	out := `Uploaded 3 of 3 assets
Deployed my-worker triggers (1.20 sec)
  https://my-worker.acme.workers.dev`

	script, _, _ := parseWranglerDeployOutput(out)
	if script != "my-worker" {
		t.Fatalf("scriptName = %q, want %q", script, "my-worker")
	}
}
