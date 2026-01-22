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

type classifierAdapter struct {
	response string
}

func (a *classifierAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	art := artifact.New(a.response, "classifier", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *classifierAdapter) Name() string { return "classifier" }

func (a *classifierAdapter) Models() []string { return []string{"cls-1"} }

type simpleAdapter struct {
	content string
}

func (a *simpleAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	art := artifact.New(a.content, "mock", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *simpleAdapter) Name() string { return "mock" }

func (a *simpleAdapter) Models() []string { return []string{"mock-1"} }

func TestRoutingDecisionRecordedWithLLM(t *testing.T) {
	llmResponse := "{\"task_type\":\"beta\",\"confidence\":0.9,\"reason\":\"tie breaker\"}"
	cfgEnabled := true
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "mock", Model: "mock-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "mock", Model: "mock-1"},
		},
		Default:                       config.RouteTarget{Adapter: "mock", Model: "mock-1"},
		ClassifierAdapter:             "classifier",
		ClassifierModel:               "cls-1",
		EnableLLMTieBreaker:           &cfgEnabled,
		ClassifierConfidenceThreshold: 0.65,
	}

	p := &Pipeline{
		Name: "routing",
		Stages: []*Stage{
			{Name: "stage", Prompt: "hello", Adapter: "mock", Model: "mock-1"},
		},
		Adapters: map[string]adapter.Adapter{
			"mock":       &simpleAdapter{content: "ok"},
			"classifier": &classifierAdapter{response: llmResponse},
		},
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
	if !record.RoutingDecision.UsedLLM {
		t.Fatalf("expected UsedLLM true")
	}
	if record.RoutingDecision.TaskType != "beta" {
		t.Fatalf("expected task_type beta, got %s", record.RoutingDecision.TaskType)
	}
	if record.RoutingDecision.Feedback == nil {
		t.Fatalf("expected post-run feedback")
	}
}
