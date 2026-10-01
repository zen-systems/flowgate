package router

import (
	"context"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/epistemic"
)

type stubAdapter struct {
	calls    int
	response string
}

func (a *stubAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	a.calls++
	art := artifact.New(a.response, "stub", model, prompt)
	return &adapter.Response{Artifact: art}, nil
}

func (a *stubAdapter) Name() string { return "stub" }

func (a *stubAdapter) Models() []string { return []string{"stub-1"} }

type stubSearch struct {
	calls int
}

func (s *stubSearch) Search(_ context.Context, query string, _ int) ([]epistemic.EvidenceRef, error) {
	s.calls++
	return []epistemic.EvidenceRef{{
		Source:  "tavily",
		Query:   query,
		Title:   "Example",
		URL:     "https://example.com",
		Snippet: "snippet",
	}}, nil
}

func TestTieBreakerRejectsUnknownTaskType(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "stub", Model: "stub-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "stub", Model: "stub-1"},
		},
		Default:           config.RouteTarget{Adapter: "stub", Model: "stub-1"},
		ClassifierAdapter: "stub",
		ClassifierModel:   "stub-1",
		Policy:            epistemic.EpistemicPolicy{Enable: true, ConfidenceThreshold: 0.7},
	}

	adapterImpl := &stubAdapter{response: "{\"task_type\":\"gamma\",\"confidence\":0.8,\"reason\":\"bad\"}"}
	decision, err := Decide(context.Background(), "alpha beta", map[string]adapter.Adapter{"stub": adapterImpl}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Final.Escalated {
		t.Fatalf("expected escalation to fail and fall back")
	}
	if decision.Final.Choice != decision.Heuristic.Choice {
		t.Fatalf("expected fallback to heuristic choice")
	}
}

func TestEscalationUsesTavilyAndClassifier(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha"}, Adapter: "stub", Model: "stub-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "stub", Model: "stub-1"},
		},
		Default:           config.RouteTarget{Adapter: "stub", Model: "stub-1"},
		ClassifierAdapter: "stub",
		ClassifierModel:   "stub-1",
		Policy:            epistemic.EpistemicPolicy{Enable: true, ConfidenceThreshold: 0.7},
		EnableTavily:      true,
	}

	adapterImpl := &stubAdapter{response: "{\"task_type\":\"beta\",\"confidence\":0.8,\"reason\":\"tie\"}"}
	search := &stubSearch{}
	decision, err := Decide(context.Background(), "alpha beta", map[string]adapter.Adapter{"stub": adapterImpl}, cfg, search)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if search.calls != 1 {
		t.Fatalf("expected tavily to be called once, got %d", search.calls)
	}
	if adapterImpl.calls != 1 {
		t.Fatalf("expected classifier to be called once, got %d", adapterImpl.calls)
	}
	if !decision.Final.Escalated {
		t.Fatalf("expected escalated true")
	}
	if len(decision.Final.EvidenceUsed) == 0 || decision.Final.EvidenceUsed[0].Source != "tavily" {
		t.Fatalf("expected tavily evidence")
	}
}

func TestNoEscalationHighConfidence(t *testing.T) {
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"alpha": {Triggers: []string{"alpha", "beta", "gamma"}, Adapter: "stub", Model: "stub-1"},
			"beta":  {Triggers: []string{"beta"}, Adapter: "stub", Model: "stub-1"},
		},
		Default:           config.RouteTarget{Adapter: "stub", Model: "stub-1"},
		ClassifierAdapter: "stub",
		ClassifierModel:   "stub-1",
		Policy:            epistemic.EpistemicPolicy{Enable: true, ConfidenceThreshold: 0.7},
		EnableTavily:      true,
	}

	adapterImpl := &stubAdapter{response: "{\"task_type\":\"beta\",\"confidence\":0.8,\"reason\":\"tie\"}"}
	search := &stubSearch{}
	decision, err := Decide(context.Background(), "alpha beta gamma", map[string]adapter.Adapter{"stub": adapterImpl}, cfg, search)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Final.Escalated {
		t.Fatalf("expected no escalation")
	}
	if search.calls != 0 || adapterImpl.calls != 0 {
		t.Fatalf("expected no external calls")
	}
}
