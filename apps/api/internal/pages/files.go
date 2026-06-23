package pages

import (
	"fmt"
	"os"
	"path/filepath"
)

// CollectFiles walks dir and returns a map of object key (forward-slash relative
// path) -> absolute file path. Directories and symlinks are skipped.
func CollectFiles(dir string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = path
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan publish folder: %w", err)
	}
	return out, nil
}
