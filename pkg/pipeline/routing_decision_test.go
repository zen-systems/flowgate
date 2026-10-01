package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type decisionAdapter struct {
	content string
}

func (a *decisionAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	art := artifact.New(a.content, "mock", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *decisionAdapter) Name() string { return "mock" }

func (a *decisionAdapter) Models() []string { return []string{"mock-1"} }

func TestRoutingDecisionRecorded(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "mock", Model: "mock-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "mock", Model: "mock-1"},
		},
		Default: config.RouteTarget{Adapter: "mock", Model: "mock-1"},
	}

	p := &Pipeline{
		Name: "routing",
		Stages: []*Stage{
			{Name: "stage", Prompt: "{{ .Input }}"},
		},
		Adapters: map[string]adapter.Adapter{"mock": &decisionAdapter{content: "ok"}},
	}

	baseDir := t.TempDir()
	_, err := Run(context.Background(), p, RunOptions{
		Input:         "alpha beta",
		EvidenceDir:   baseDir,
		RoutingConfig: cfg,
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected run directory")
	}
	runDir := filepath.Join(baseDir, entries[0].Name())
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}

	var record evidence.RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal run.json: %v", err)
	}
	if record.RoutingDecision == nil {
		t.Fatalf("expected routing decision")
	}
	decision := record.RoutingDecision.Final
	if decision.Kind != "route" {
		t.Fatalf("expected kind=route")
	}
	if len(decision.Candidates) < 2 {
		t.Fatalf("expected multiple candidates")
	}
	if decision.Choice == "" {
		t.Fatalf("expected choice")
	}
	if decision.Confidence != 0 {
		t.Fatalf("expected confidence 0 for tied scores, got %.2f", decision.Confidence)
	}
	if !decision.NeedsEgress {
		t.Fatalf("expected needs_egress true")
	}
	if decision.Escalated {
		t.Fatalf("expected escalated=false")
	}
}

func TestRoutingDecisionRecordedWithStageOverride(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "mock", Model: "mock-1"},
		},
		Default: config.RouteTarget{Adapter: "mock", Model: "mock-1"},
	}

	p := &Pipeline{
		Name: "routing",
		Stages: []*Stage{
			{Name: "stage", Prompt: "hello", Adapter: "mock", Model: "mock-1"},
		},
		Adapters: map[string]adapter.Adapter{"mock": &decisionAdapter{content: "ok"}},
	}

	baseDir := t.TempDir()
	_, err := Run(context.Background(), p, RunOptions{
		Input:         "alpha",
		EvidenceDir:   baseDir,
		RoutingConfig: cfg,
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	record := readRunRecordOverride(t, baseDir)
	if record.RoutingDecision == nil {
		t.Fatalf("expected routing decision")
	}
}

func readRunRecordOverride(t *testing.T, baseDir string) evidence.RunRecord {
	t.Helper()
	entries, err := os.ReadDir(baseDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected run directory")
	}
	runDir := filepath.Join(baseDir, entries[0].Name())
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}

	var record evidence.RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal run.json: %v", err)
	}
	return record
}
