package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
)

type staticAdapter struct {
	content string
}

func (a *staticAdapter) Generate(_ context.Context, model string, prompt string) (*artifact.Artifact, error) {
	return artifact.New(a.content, "static", model, prompt), nil
}

func (a *staticAdapter) Name() string { return "static" }

func (a *staticAdapter) Models() []string { return []string{"mock-1"} }

func TestCommandGatePolicyBlocksShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	workspace := t.TempDir()
	blockedFile := filepath.Join(workspace, "blocked.txt")

	p := &Pipeline{
		Name: "policy",
		Gates: map[string]GateDefinition{
			"block": {
				Type:       "command",
				Capability: "go_test",
				Command:    []string{"sh", "-c", "echo blocked > blocked.txt"},
			},
		},
		Stages: []*Stage{
			{
				Name:       "stage",
				Prompt:     "hello",
				Adapter:    "static",
				Model:      "mock-1",
				Gates:      []string{"block"},
				MaxRetries: 0,
			},
		},
		Adapters: map[string]adapter.Adapter{"static": &staticAdapter{content: "ok"}},
	}

	_, err := Run(context.Background(), p, RunOptions{WorkspacePath: workspace, EvidenceDir: t.TempDir(), Input: "input"})
	if err == nil {
		t.Fatalf("expected policy block error")
	}
	if _, err := os.Stat(blockedFile); !os.IsNotExist(err) {
		t.Fatalf("expected blocked command not to execute")
	}
}

func TestCommandGatePolicyAllowsGofmt(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available")
	}

	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "main.go")
	unformatted := []byte("package main\nfunc main(){\n}\n")
	if err := os.WriteFile(filePath, unformatted, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	p := &Pipeline{
		Name: "policy-allow",
		Gates: map[string]GateDefinition{
			"fmt": {
				Type:       "command",
				Capability: "gofmt",
				Command:    []string{"gofmt", "-w", "main.go"},
			},
		},
		Stages: []*Stage{
			{
				Name:       "stage",
				Prompt:     "hello",
				Adapter:    "static",
				Model:      "mock-1",
				Gates:      []string{"fmt"},
				MaxRetries: 0,
			},
		},
		Adapters: map[string]adapter.Adapter{"static": &staticAdapter{content: "ok"}},
	}

	_, err := Run(context.Background(), p, RunOptions{WorkspacePath: workspace, EvidenceDir: t.TempDir(), Input: "input"})
	if err != nil {
		t.Fatalf("expected gofmt gate to pass: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) == string(unformatted) {
		t.Fatalf("expected gofmt to modify file")
	}
}
