package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

func TestAttemptEvidenceBlobs(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	workspaceDir := t.TempDir()
	baseEvidence := t.TempDir()

	p := &Pipeline{
		Name: "attempts",
		Gates: map[string]GateDefinition{
			"flaky": {
				Type:      "command",
				Command:   []string{"sh", "-c", "if [ -f .attempt ]; then exit 0; else touch .attempt; exit 1; fi"},
				DenyShell: boolPtr(false),
			},
		},
		Stages: []*Stage{
			{
				Name:       "stage",
				Prompt:     "hello",
				Adapter:    "mock",
				Model:      "mock-1",
				Gates:      []string{"flaky"},
				MaxRetries: 1,
			},
		},
		Adapters: map[string]adapter.Adapter{"mock": adapter.NewMockAdapter()},
	}

	result, err := Run(context.Background(), p, RunOptions{
		Input:         "input",
		EvidenceDir:   baseEvidence,
		WorkspacePath: workspaceDir,
		ApplyApproved: true,
	})
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

	if len(record.Attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(record.Attempts))
	}

	for i, attempt := range record.Attempts {
		if attempt.PromptRef == "" || attempt.OutputRef == "" {
			t.Fatalf("attempt %d missing refs", i+1)
		}
		promptPath := filepath.Join(result.EvidenceDir, attempt.PromptRef)
		if _, err := os.Stat(promptPath); err != nil {
			t.Fatalf("attempt %d prompt blob missing: %v", i+1, err)
		}
		outputPath := filepath.Join(result.EvidenceDir, attempt.OutputRef)
		if _, err := os.Stat(outputPath); err != nil {
			t.Fatalf("attempt %d output blob missing: %v", i+1, err)
		}
	}
}
