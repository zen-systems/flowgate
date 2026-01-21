package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestEvidenceWriter(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, "run-123")
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	run := RunRecord{
		ID:           "run-123",
		Timestamp:    time.Now().UTC(),
		PipelineFile: "pipeline.yaml",
		InputHash:    "abc",
		Workspace:    dir,
	}
	if err := writer.WriteRun(run); err != nil {
		t.Fatalf("write run: %v", err)
	}

	stage := StageRecord{
		Name:    "stage1",
		Adapter: "mock",
		Model:   "mock-1",
		Output:  "ok",
	}
	if err := writer.WriteStage(stage); err != nil {
		t.Fatalf("write stage: %v", err)
	}

	if err := writer.WriteGateLog("stage1", "gate1", "stdout"); err != nil {
		t.Fatalf("write gate log: %v", err)
	}

	if _, err := os.Stat(filepath.Join(writer.RunDir(), "run.json")); err != nil {
		t.Fatalf("missing run.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(writer.RunDir(), "stages", "stage1.json")); err != nil {
		t.Fatalf("missing stage file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(writer.RunDir(), "gates", "stage1-gate1.log")); err != nil {
		t.Fatalf("missing gate log: %v", err)
	}

	if runtime.GOOS != "windows" {
		assertPerm(t, writer.RunDir(), 0700)
		assertPerm(t, filepath.Join(writer.RunDir(), "stages"), 0700)
		assertPerm(t, filepath.Join(writer.RunDir(), "gates"), 0700)
		assertPerm(t, filepath.Join(writer.RunDir(), "blobs"), 0700)
		assertPerm(t, filepath.Join(writer.RunDir(), "run.json"), 0600)
		assertPerm(t, filepath.Join(writer.RunDir(), "stages", "stage1.json"), 0600)
		assertPerm(t, filepath.Join(writer.RunDir(), "gates", "stage1-gate1.log"), 0600)
	}
}

func TestWriteBlob(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, "run1")
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	content := []byte("hello")
	sum := sha256.Sum256(content)
	expectedSha := hex.EncodeToString(sum[:])

	ref, sha, err := writer.WriteBlob("prompt", content)
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	if sha != expectedSha {
		t.Fatalf("sha mismatch: %s", sha)
	}

	blobPath := filepath.Join(writer.RunDir(), ref)
	data, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("content mismatch: %q", string(data))
	}
	if runtime.GOOS != "windows" {
		assertPerm(t, blobPath, 0600)
	}

	ref2, sha2, err := writer.WriteBlob("prompt", content)
	if err != nil {
		t.Fatalf("write blob again: %v", err)
	}
	if ref2 != ref || sha2 != sha {
		t.Fatalf("expected same ref and sha")
	}
}

func TestWriteBlobKindSanitization(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, "run2")
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	ref, _, err := writer.WriteBlob("Prompt 123/../", []byte("x"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}

	if !strings.HasPrefix(ref, "blobs/") {
		t.Fatalf("expected blobs prefix: %s", ref)
	}
	if strings.Count(ref, "/") != 1 {
		t.Fatalf("unexpected path separators in ref: %s", ref)
	}

	kindSegment := strings.TrimPrefix(ref, "blobs/")
	kindSegment = strings.TrimSuffix(kindSegment, filepath.Ext(kindSegment))
	parts := strings.SplitN(kindSegment, "-", 2)
	if len(parts) == 0 {
		t.Fatalf("missing kind segment in ref: %s", ref)
	}

	kind := parts[0]
	for _, r := range kind {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			t.Fatalf("invalid kind character: %q", r)
		}
	}
}

func TestWriteBlobKindFallback(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, "run3")
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	ref, _, err := writer.WriteBlob("!!!", []byte("y"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}

	if !strings.HasPrefix(ref, "blobs/blob-") {
		t.Fatalf("expected blob kind fallback in ref: %s", ref)
	}
}

func assertPerm(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode().Perm() != expected {
		t.Fatalf("expected %s mode %o, got %o", path, expected, info.Mode().Perm())
	}
}
