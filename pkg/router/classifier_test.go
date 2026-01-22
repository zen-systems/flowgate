package router

import (
	"context"
	"math"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/config"
)

type countingAdapter struct {
	calls    int
	response string
}

func (a *countingAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	a.calls++
	art := artifact.New(a.response, "counting", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *countingAdapter) Name() string { return "counting" }

func (a *countingAdapter) Models() []string { return []string{"mock-1"} }

func TestHeuristicDecisionConfidence(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}, Adapter: "a", Model: "m1"},
			"beta":  {Triggers: []string{"alpha", "beta"}, Adapter: "b", Model: "m2"},
		},
		Default: config.RouteTarget{Adapter: "default", Model: "d"},
	}

	decision := HeuristicDecision("alpha beta gamma", cfg)
	if decision.TaskType != "alpha" {
		t.Fatalf("expected alpha, got %s", decision.TaskType)
	}
	if len(decision.Candidates) < 2 {
		t.Fatalf("expected candidates")
	}
	if decision.Candidates[0].Score != 3 || decision.Candidates[1].Score != 2 {
		t.Fatalf("unexpected scores: %+v", decision.Candidates)
	}

	want := 0.55
	if math.Abs(decision.Confidence-want) > 0.02 {
		t.Fatalf("confidence mismatch: got %.2f want %.2f", decision.Confidence, want)
	}
}

func TestHeuristicDecisionStrongMatch(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}, Adapter: "a", Model: "m1"},
			"beta":  {Triggers: []string{"delta"}, Adapter: "b", Model: "m2"},
		},
		Default: config.RouteTarget{Adapter: "default", Model: "d"},
	}

	decision := HeuristicDecision("alpha beta gamma", cfg)
	if decision.TaskType != "alpha" {
		t.Fatalf("expected alpha, got %s", decision.TaskType)
	}
	if decision.Confidence < 0.9 {
		t.Fatalf("expected high confidence, got %.2f", decision.Confidence)
	}
}

func TestHeuristicDecisionNoMatches(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "a", Model: "m1"},
		},
		Default: config.RouteTarget{Adapter: "default", Model: "d"},
	}

	decision := HeuristicDecision("no matches here", cfg)
	if decision.TaskType != "default" {
		t.Fatalf("expected default, got %s", decision.TaskType)
	}
	if decision.Confidence != 0 {
		t.Fatalf("expected confidence 0, got %.2f", decision.Confidence)
	}
	if len(decision.Candidates) != 0 {
		t.Fatalf("expected no candidates")
	}
}

func TestTieBreakerGating(t *testing.T) {
	adapterImpl := &countingAdapter{response: "{}"}
	enabled := true
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}, Adapter: "a", Model: "m1"},
			"beta":  {Triggers: []string{"alpha"}, Adapter: "b", Model: "m2"},
		},
		Default:                       config.RouteTarget{Adapter: "default", Model: "d"},
		ClassifierAdapter:             "classifier",
		ClassifierModel:               "mock-1",
		EnableLLMTieBreaker:           &enabled,
		ClassifierConfidenceThreshold: 0.65,
	}

	classifier := NewClassifier(map[string]adapter.Adapter{"classifier": adapterImpl}, cfg)
	decision, err := classifier.Classify(context.Background(), "alpha beta gamma")
	if err != nil {
		// Should not require LLM, but no hard error expected either
	}
	if decision.UsedLLM {
		t.Fatalf("expected no LLM usage")
	}
	if adapterImpl.calls != 0 {
		t.Fatalf("expected classifier not called")
	}

	missingCfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "a", Model: "m1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "b", Model: "m2"},
		},
		Default:                       config.RouteTarget{Adapter: "default", Model: "d"},
		ClassifierAdapter:             "missing",
		ClassifierModel:               "mock-1",
		EnableLLMTieBreaker:           &enabled,
		ClassifierConfidenceThreshold: 0.65,
	}

	classifier = NewClassifier(map[string]adapter.Adapter{}, missingCfg)
	decision, _ = classifier.Classify(context.Background(), "alpha beta")
	if decision.UsedLLM {
		t.Fatalf("expected no LLM when classifier missing")
	}
}
