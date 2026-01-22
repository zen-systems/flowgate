package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type largeOutputAdapter struct {
	content string
}

func (a *largeOutputAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	art := artifact.New(a.content, "fake", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *largeOutputAdapter) Name() string {
	return "fake"
}

func (a *largeOutputAdapter) Models() []string {
	return []string{"fake-1"}
}

func TestStageRecordOutputBlob(t *testing.T) {
	baseDir := t.TempDir()
	output := strings.Repeat("x", 5000)
	fake := &largeOutputAdapter{content: output}

	p := &Pipeline{
		Name: "blob-test",
		Stages: []*Stage{
			{
				Name:   "stage",
				Prompt: "hello",
				Model:  "fake-1",
			},
		},
		Adapters: map[string]adapter.Adapter{"fake": fake},
	}

	result, err := Run(context.Background(), p, RunOptions{Input: "input", EvidenceDir: baseDir})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	stagePath := filepath.Join(result.EvidenceDir, "stages", "stage.json")
	data, err := os.ReadFile(stagePath)
	if err != nil {
		t.Fatalf("read stage record: %v", err)
	}

	var record evidence.StageRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal stage record: %v", err)
	}

	if len(record.Output) >= len(output) {
		t.Fatalf("expected truncated output preview")
	}
	if record.OutputRef == "" || record.OutputLen != len(output) {
		t.Fatalf("missing output ref or length")
	}
	if record.PromptRef == "" || record.PromptLen == 0 {
		t.Fatalf("missing prompt ref or length")
	}

	blobPath := filepath.Join(result.EvidenceDir, record.OutputRef)
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(blobData) != output {
		t.Fatalf("blob content mismatch")
	}

	promptPath := filepath.Join(result.EvidenceDir, record.PromptRef)
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt blob: %v", err)
	}
	if string(promptData) != "hello" {
		t.Fatalf("prompt blob content mismatch")
	}
}
