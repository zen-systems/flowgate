package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneToTemp(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".flowgate", "runs", "run1"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".flowgate", "runs", "run1", "ignored.txt"), []byte("skip"), 0644); err != nil {
		t.Fatalf("write ignored: %v", err)
	}

	clone, cleanup, err := CloneToTemp(root)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(filepath.Join(clone, "hello.txt"))
	if err != nil {
		t.Fatalf("read clone: %v", err)
	}
	if string(data) != "hi" {
		t.Fatalf("unexpected clone content: %q", string(data))
	}

	if _, err := os.Stat(filepath.Join(clone, ".flowgate", "runs")); !os.IsNotExist(err) {
		t.Fatalf("expected .flowgate/runs to be skipped")
	}
}
