package router

import (
	"math"
	"testing"

	"github.com/zen-systems/flowgate/pkg/config"
)

func TestHeuristicDecisionConfidence(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}},
			"beta":  {Triggers: []string{"alpha", "beta"}},
		},
		Default: config.RouteTarget{Adapter: "default", Model: "d"},
	}

	decision := HeuristicDecision("alpha beta gamma", cfg)
	if decision.Choice != "alpha" {
		t.Fatalf("expected alpha, got %s", decision.Choice)
	}
	if len(decision.Candidates) != 2 {
		t.Fatalf("expected 2 candidates")
	}
	if decision.Candidates[0] != "alpha" || decision.Candidates[1] != "beta" {
		t.Fatalf("unexpected candidate order: %v", decision.Candidates)
	}

	want := 0.53
	if math.Abs(decision.Confidence-want) > 0.03 {
		t.Fatalf("confidence mismatch: got %.2f want %.2f", decision.Confidence, want)
	}
}

func TestHeuristicDecisionStrongMatch(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}},
			"beta":  {Triggers: []string{"delta"}},
		},
		Default: config.RouteTarget{Adapter: "default", Model: "d"},
	}

	decision := HeuristicDecision("alpha beta gamma", cfg)
	if decision.Choice != "alpha" {
		t.Fatalf("expected alpha, got %s", decision.Choice)
	}
	if decision.Confidence < 0.9 {
		t.Fatalf("expected high confidence, got %.2f", decision.Confidence)
	}
}

func TestHeuristicDecisionNoMatches(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}},
		},
		Default: config.RouteTarget{Adapter: "default", Model: "d"},
	}

	decision := HeuristicDecision("no matches here", cfg)
	if decision.Choice != "default" {
		t.Fatalf("expected default, got %s", decision.Choice)
	}
	if decision.Confidence != 0 {
		t.Fatalf("expected confidence 0, got %.2f", decision.Confidence)
	}
	if len(decision.Candidates) != 0 {
		t.Fatalf("expected no candidates")
	}
}
