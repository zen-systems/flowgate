package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type fileBlockAdapter struct {
	content string
}

func (a *fileBlockAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	art := artifact.New(a.content, "fileblock", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *fileBlockAdapter) Name() string {
	return "fileblock"
}

func (a *fileBlockAdapter) Models() []string {
	return []string{"fileblock-1"}
}

func TestApplyUsesTempWorkspaceByDefault(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	realWorkspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(realWorkspace, "hello.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("write real workspace: %v", err)
	}

	content := "// file: hello.txt\nmodified\n"
	fakeAdapter := &fileBlockAdapter{content: content}

	p := &Pipeline{
		Name: "dry-run",
		Gates: map[string]GateDefinition{
			"check": {
				Type:      "command",
				Command:   []string{"sh", "-c", "grep -q modified hello.txt"},
				DenyShell: boolPtr(false),
			},
		},
		Stages: []*Stage{
			{
				Name:   "stage",
				Prompt: "apply",
				Model:  "fileblock-1",
				Apply:  true,
				Gates:  []string{"check"},
			},
		},
		Adapters: map[string]adapter.Adapter{"fileblock": fakeAdapter},
	}

	result, err := Run(context.Background(), p, RunOptions{
		Input:         "input",
		WorkspacePath: realWorkspace,
		EvidenceDir:   t.TempDir(),
		ApplyApproved: true,
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(realWorkspace, "hello.txt"))
	if err != nil {
		t.Fatalf("read real workspace: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("real workspace modified")
	}

	stagePath := filepath.Join(result.EvidenceDir, "stages", "stage.json")
	stageData, err := os.ReadFile(stagePath)
	if err != nil {
		t.Fatalf("read stage record: %v", err)
	}

	var record evidence.StageRecord
	if err := json.Unmarshal(stageData, &record); err != nil {
		t.Fatalf("unmarshal stage record: %v", err)
	}
	if len(record.Attempts) != 1 {
		t.Fatalf("expected 1 attempt")
	}
	attempt := record.Attempts[0]
	if attempt.WorkspaceMode != "temp" {
		t.Fatalf("expected temp workspace mode")
	}
	if attempt.WorkspaceUsed == "" || attempt.WorkspaceUsed == realWorkspace {
		t.Fatalf("expected temp workspace path")
	}
}

func TestApplyForReal(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	realWorkspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(realWorkspace, "hello.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("write real workspace: %v", err)
	}

	content := "// file: hello.txt\nmodified\n"
	fakeAdapter := &fileBlockAdapter{content: content}

	p := &Pipeline{
		Name: "apply-real",
		Gates: map[string]GateDefinition{
			"check": {
				Type:      "command",
				Command:   []string{"sh", "-c", "grep -q modified hello.txt"},
				DenyShell: boolPtr(false),
			},
		},
		Stages: []*Stage{
			{
				Name:   "stage",
				Prompt: "apply",
				Model:  "fileblock-1",
				Apply:  true,
				Gates:  []string{"check"},
			},
		},
		Adapters: map[string]adapter.Adapter{"fileblock": fakeAdapter},
	}

	result, err := Run(context.Background(), p, RunOptions{
		Input:         "input",
		WorkspacePath: realWorkspace,
		EvidenceDir:   t.TempDir(),
		ApplyForReal:  true,
		ApplyApproved: true,
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(realWorkspace, "hello.txt"))
	if err != nil {
		t.Fatalf("read real workspace: %v", err)
	}
	if strings.TrimSpace(string(data)) != "modified" {
		t.Fatalf("real workspace not modified")
	}

	stagePath := filepath.Join(result.EvidenceDir, "stages", "stage.json")
	stageData, err := os.ReadFile(stagePath)
	if err != nil {
		t.Fatalf("read stage record: %v", err)
	}

	var record evidence.StageRecord
	if err := json.Unmarshal(stageData, &record); err != nil {
		t.Fatalf("unmarshal stage record: %v", err)
	}
	if len(record.Attempts) != 1 {
		t.Fatalf("expected 1 attempt")
	}
	attempt := record.Attempts[0]
	if attempt.WorkspaceMode != "real" {
		t.Fatalf("expected real workspace mode")
	}
	if attempt.WorkspaceUsed != realWorkspace {
		t.Fatalf("expected real workspace path")
	}
}
