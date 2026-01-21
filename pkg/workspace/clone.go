package workspace

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CloneToTemp copies a workspace directory tree into a temp directory.
func CloneToTemp(src string) (tempDir string, cleanup func() error, err error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("workspace path is not a directory")
	}

	tempDir, err = os.MkdirTemp("", "flowgate-workspace-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() error { return os.RemoveAll(tempDir) }

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkip(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		destPath := filepath.Join(tempDir, rel)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, destPath, info.Mode())
	})
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return tempDir, cleanup, nil
}

func shouldSkip(rel string, d fs.DirEntry) bool {
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 2 && parts[0] == ".flowgate" && parts[1] == "runs" {
		return true
	}
	return false
}

func copyFile(src, dest string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
