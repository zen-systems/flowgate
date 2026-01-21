package evidence

import (
	"os"
	"path/filepath"
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
}
