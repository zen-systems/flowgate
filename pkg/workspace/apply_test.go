package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyOutputUnifiedDiff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	diff := "--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n a\n-b\n+bee\n c\n"
	if _, err := ApplyOutput(dir, diff); err != nil {
		t.Fatalf("apply diff: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "a\nbee\nc\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestApplyOutputFileBlocks(t *testing.T) {
	dir := t.TempDir()
	content := "// file: hello.txt\nhello world\n"
	if _, err := ApplyOutput(dir, content); err != nil {
		t.Fatalf("apply blocks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}
