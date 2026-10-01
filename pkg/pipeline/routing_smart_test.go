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
	"github.com/zen-systems/flowgate/pkg/epistemic"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type countingAdapter struct {
	name     string
	calls    int
	response string
}

func (a *countingAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	a.calls++
	content := "ok"
	if a.response != "" {
		content = a.response
	}
	art := artifact.New(content, a.name, model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *countingAdapter) Name() string { return a.name }

func (a *countingAdapter) Models() []string { return []string{"mock-1"} }

type countingSearch struct {
	calls int
}

func (s *countingSearch) Search(_ context.Context, query string, _ int) ([]epistemic.EvidenceRef, error) {
	s.calls++
	return []epistemic.EvidenceRef{{Source: "tavily", Query: query}}, nil
}

func TestSmartRoutingHighConfidenceNoClassifier(t *testing.T) {
	alphaAdapter := &countingAdapter{name: "alpha"}
	classifier := &countingAdapter{name: "classifier"}
	search := &countingSearch{}
	enabled := true
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}, Adapter: "alpha", Model: "mock-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "beta", Model: "mock-1"},
		},
		Default:           config.RouteTarget{Adapter: "alpha", Model: "mock-1"},
		ClassifierAdapter: "classifier",
		ClassifierModel:   "mock-1",
		Policy:            epistemic.EpistemicPolicy{Enable: true, ConfidenceThreshold: 0.7},
		EnableTavily:      true,
	}
	cfg.EnableLLMTieBreaker = &enabled

	p := &Pipeline{
		Name: "routing",
		Stages: []*Stage{
			{Name: "stage", Prompt: "alpha beta gamma"},
		},
		Adapters: map[string]adapter.Adapter{
			"alpha":      alphaAdapter,
			"classifier": classifier,
		},
	}

	baseDir := t.TempDir()
	_, err := Run(context.Background(), p, RunOptions{
		Input:         "alpha beta gamma",
		EvidenceDir:   baseDir,
		RoutingConfig: cfg,
		WebSearch:     search,
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if classifier.calls != 0 {
		t.Fatalf("expected classifier not called")
	}
	if search.calls != 0 {
		t.Fatalf("expected web search not called")
	}

	runRecord := readRunRecord(t, baseDir)
	if runRecord.RoutingDecision == nil {
		t.Fatalf("expected routing decision")
	}
	if runRecord.RoutingDecision.Final.Escalated {
		t.Fatalf("expected no escalation")
	}
}

func TestSmartRoutingLowConfidenceUsesClassifier(t *testing.T) {
	alphaAdapter := &countingAdapter{name: "alpha"}
	betaAdapter := &countingAdapter{name: "beta"}
	classifier := &countingAdapter{name: "classifier"}
	search := &countingSearch{}
	enabled := true
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "alpha", Model: "mock-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "beta", Model: "mock-1"},
		},
		Default:           config.RouteTarget{Adapter: "alpha", Model: "mock-1"},
		ClassifierAdapter: "classifier",
		ClassifierModel:   "mock-1",
		Policy:            epistemic.EpistemicPolicy{Enable: true, ConfidenceThreshold: 0.7},
		EnableTavily:      true,
	}
	cfg.EnableLLMTieBreaker = &enabled

	classifier.response = "{\"task_type\":\"beta\",\"confidence\":0.8,\"reason\":\"tie\"}"

	p := &Pipeline{
		Name: "routing",
		Stages: []*Stage{
			{Name: "stage", Prompt: "alpha beta"},
		},
		Adapters: map[string]adapter.Adapter{
			"alpha":      alphaAdapter,
			"beta":       betaAdapter,
			"classifier": classifier,
		},
	}

	baseDir := t.TempDir()
	_, err := Run(context.Background(), p, RunOptions{
		Input:         "alpha beta",
		EvidenceDir:   baseDir,
		RoutingConfig: cfg,
		WebSearch:     search,
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if classifier.calls != 1 {
		t.Fatalf("expected classifier called once, got %d", classifier.calls)
	}
	if search.calls != 1 {
		t.Fatalf("expected web search called once, got %d", search.calls)
	}
	if betaAdapter.calls != 1 || alphaAdapter.calls != 0 {
		t.Fatalf("expected beta adapter to be used")
	}

	runRecord := readRunRecord(t, baseDir)
	if runRecord.RoutingDecision == nil {
		t.Fatalf("expected routing decision")
	}
	if runRecord.RoutingDecision.Final.Choice != "beta" {
		t.Fatalf("expected final choice beta")
	}
	if !runRecord.RoutingDecision.Final.Escalated {
		t.Fatalf("expected escalation")
	}
}

func readRunRecord(t *testing.T, baseDir string) evidence.RunRecord {
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
