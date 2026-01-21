package pipeline

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type fixedAdapter struct {
	content string
}

func (a *fixedAdapter) Generate(_ context.Context, model string, prompt string) (*artifact.Artifact, error) {
	return artifact.New(a.content, "fixed", model, prompt), nil
}

func (a *fixedAdapter) Name() string { return "fixed" }

func (a *fixedAdapter) Models() []string { return []string{"mock-1"} }

type changingAdapter struct {
	contents []string
	index    int
}

func (a *changingAdapter) Generate(_ context.Context, model string, prompt string) (*artifact.Artifact, error) {
	if a.index >= len(a.contents) {
		return artifact.New("fallback", "changing", model, prompt), nil
	}
	content := a.contents[a.index]
	a.index++
	return artifact.New(content, "changing", model, prompt), nil
}

func (a *changingAdapter) Name() string { return "changing" }

func (a *changingAdapter) Models() []string { return []string{"mock-1"} }

func TestRepairLoopDetection(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	writer, err := evidence.NewWriter(t.TempDir(), "run1")
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	p := &Pipeline{
		Name: "loop",
		Gates: map[string]GateDefinition{
			"fail": {
				Type:      "command",
				Command:   []string{"sh", "-c", "exit 1"},
				DenyShell: boolPtr(false),
			},
		},
	}

	stage := &Stage{
		Name:       "stage",
		Prompt:     "hello",
		Adapter:    "fixed",
		Model:      "mock-1",
		Gates:      []string{"fail"},
		MaxRetries: 2,
	}

	_, stageRecord, err := runStage(
		context.Background(),
		writer,
		stage,
		map[string]adapter.Adapter{"fixed": &fixedAdapter{content: "same"}},
		p,
		"input",
		t.TempDir(),
		false,
		true,
		map[string]ArtifactTemplateData{},
		map[string]map[string]string{},
	)
	if err == nil || !strings.Contains(err.Error(), "repair loop detected") {
		t.Fatalf("expected repair loop error, got %v", err)
	}
	if stageRecord == nil || len(stageRecord.Attempts) != 3 {
		t.Fatalf("expected 3 attempts")
	}
}

func TestRepairLoopAllowsChangedOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	writer, err := evidence.NewWriter(t.TempDir(), "run2")
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	p := &Pipeline{
		Name: "loop",
		Gates: map[string]GateDefinition{
			"fail": {
				Type:      "command",
				Command:   []string{"sh", "-c", "exit 1"},
				DenyShell: boolPtr(false),
			},
		},
	}

	stage := &Stage{
		Name:       "stage",
		Prompt:     "hello",
		Adapter:    "changing",
		Model:      "mock-1",
		Gates:      []string{"fail"},
		MaxRetries: 1,
	}

	_, stageRecord, err := runStage(
		context.Background(),
		writer,
		stage,
		map[string]adapter.Adapter{"changing": &changingAdapter{contents: []string{"one", "two"}}},
		p,
		"input",
		t.TempDir(),
		false,
		true,
		map[string]ArtifactTemplateData{},
		map[string]map[string]string{},
	)
	if err == nil {
		t.Fatalf("expected gate failure")
	}
	if strings.Contains(err.Error(), "repair loop detected") {
		t.Fatalf("unexpected repair loop error")
	}
	if stageRecord == nil || len(stageRecord.Attempts) != 2 {
		t.Fatalf("expected 2 attempts")
	}
}
