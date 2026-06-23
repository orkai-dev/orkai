package aws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectFiles_skipsGitDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "objects", "abc"), []byte("object"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := collectFiles(dir)
	if err != nil {
		t.Fatalf("collectFiles: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if _, ok := files["index.html"]; !ok {
		t.Fatalf("expected index.html in result, got %v", files)
	}
	if _, ok := files[".gitignore"]; !ok {
		t.Fatalf("expected .gitignore in result, got %v", files)
	}
	for key := range files {
		if strings.HasPrefix(key, ".git/") {
			t.Fatalf("unexpected .git file in result: %q", key)
		}
	}
}
