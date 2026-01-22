package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type budgetAdapter struct {
	calls int
	usage adapter.Usage
}

func (a *budgetAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	a.calls++
	art := artifact.New("ok", "budget", model, prompt)
	return &adapter.Response{Artifact: art, Usage: &a.usage}, nil
}

func (a *budgetAdapter) Name() string { return "budget" }

func (a *budgetAdapter) Models() []string { return []string{"budget-1"} }

type transientAdapter struct {
	failures int
	calls    int
}

func (a *transientAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	a.calls++
	if a.calls <= a.failures {
		return nil, &adapter.AdapterError{Status: 429, Temporary: true, Err: fmt.Errorf("rate limit")}
	}
	art := artifact.New("ok", "transient", model, prompt)
	return &adapter.Response{Artifact: art, Usage: &adapter.Usage{PromptTokens: 10}}, nil
}

func (a *transientAdapter) Name() string { return "transient" }

func (a *transientAdapter) Models() []string { return []string{"mock-1"} }

type failingAdapter struct{}

func (a *failingAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	return nil, fmt.Errorf("hard failure")
}

func (a *failingAdapter) Name() string { return "primary" }

func (a *failingAdapter) Models() []string { return []string{"mock-1"} }

type fallbackAdapter struct{}

func (a *fallbackAdapter) Generate(_ context.Context, model string, prompt string) (*adapter.Response, error) {
	art := artifact.New("ok", "secondary", model, prompt)
	return &adapter.Response{Artifact: art, Usage: &adapter.Usage{PromptTokens: 5}}, nil
}

func (a *fallbackAdapter) Name() string { return "secondary" }

func (a *fallbackAdapter) Models() []string { return []string{"mock-1"} }

func TestEstimateCostAndTotals(t *testing.T) {
	pricing := config.PricingConfig{
		"openai": {
			"gpt-1": {
				PromptPer1K:     0.15,
				CompletionPer1K: 0.60,
			},
		},
	}

	usage := adapter.Usage{PromptTokens: 1000, CompletionTokens: 500}
	cost, ok := estimateCost(pricing, "openai", "gpt-1", usage)
	if !ok {
		t.Fatalf("expected pricing match")
	}
	want := 0.15 + 0.30
	if math.Abs(cost.Amount-want) > 1e-6 {
		t.Fatalf("cost amount mismatch: got %.4f want %.4f", cost.Amount, want)
	}

	tracker := newCostTracker(&config.RoutingConfig{Pricing: pricing}, 0)
	tracker.recordReports([]adapter.CallReport{{
		Adapter: "openai",
		Model:   "gpt-1",
		Usage:   usage,
		Cost:    cost,
	}})
	tracker.recordReports([]adapter.CallReport{{
		Adapter: "openai",
		Model:   "gpt-1",
		Usage:   usage,
		Cost:    cost,
	}})

	report := tracker.report()
	if report.TotalUsage.PromptTokens != 2000 {
		t.Fatalf("expected prompt tokens to sum to 2000, got %d", report.TotalUsage.PromptTokens)
	}
	if math.Abs(report.TotalAmount-(want*2)) > 1e-6 {
		t.Fatalf("expected total cost %.4f, got %.4f", want*2, report.TotalAmount)
	}
}

func TestBudgetEnforcementStopsSecondCall(t *testing.T) {
	pricing := config.PricingConfig{
		"budget": {
			"budget-1": {
				PromptPer1K: 1.0,
			},
		},
	}
	cfg := &config.RoutingConfig{Pricing: pricing}
	adapterImpl := &budgetAdapter{usage: adapter.Usage{PromptTokens: 1000}}

	p := &Pipeline{
		Name: "budget",
		Stages: []*Stage{
			{Name: "one", Prompt: "hello", Adapter: "budget", Model: "budget-1"},
			{Name: "two", Prompt: "world", Adapter: "budget", Model: "budget-1"},
		},
		Adapters: map[string]adapter.Adapter{"budget": adapterImpl},
	}

	baseDir := t.TempDir()
	_, err := Run(context.Background(), p, RunOptions{
		Input:         "input",
		EvidenceDir:   baseDir,
		RoutingConfig: cfg,
		MaxBudgetUSD:  1.5,
	})
	if err == nil {
		t.Fatalf("expected budget error")
	}
	if adapterImpl.calls != 1 {
		t.Fatalf("expected 1 adapter call, got %d", adapterImpl.calls)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected evidence run directory")
	}
	runDir := filepath.Join(baseDir, entries[0].Name())
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}
	var record evidence.RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal run record: %v", err)
	}
	if record.CostReport == nil || record.CostReport.Budget == nil {
		t.Fatalf("expected budget status in cost report")
	}
	if !record.CostReport.Budget.Exceeded || record.CostReport.Budget.Reason == "" {
		t.Fatalf("expected budget exceeded with reason")
	}
}

func TestRetryWithTransientErrors(t *testing.T) {
	cfg := &config.RoutingConfig{
		Retry: config.RetryConfig{
			MaxRetries:    2,
			BaseBackoffMs: 1,
			MaxBackoffMs:  2,
		},
	}
	adapterImpl := &transientAdapter{failures: 2}
	resp, reports, err := callAdapterWithPolicy(
		context.Background(),
		map[string]adapter.Adapter{"transient": adapterImpl},
		"transient",
		"mock-1",
		"prompt",
		cfg,
		newCostTracker(cfg, 0),
	)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if resp == nil || resp.Artifact == nil {
		t.Fatalf("expected response artifact")
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 call report, got %d", len(reports))
	}
	if reports[0].Retries != 2 {
		t.Fatalf("expected 2 retries, got %d", reports[0].Retries)
	}
}

func TestFallbackAdapterUsedOnFailure(t *testing.T) {
	cfg := &config.RoutingConfig{
		Retry: config.RetryConfig{MaxRetries: 0},
		Fallback: config.FallbackConfig{
			AllowFallback: true,
			FallbackChain: map[string][]config.RouteTarget{
				"primary/mock-1": {{Adapter: "secondary", Model: "mock-1"}},
			},
		},
	}

	resp, reports, err := callAdapterWithPolicy(
		context.Background(),
		map[string]adapter.Adapter{
			"primary":   &failingAdapter{},
			"secondary": &fallbackAdapter{},
		},
		"primary",
		"mock-1",
		"prompt",
		cfg,
		newCostTracker(cfg, 0),
	)
	if err != nil {
		t.Fatalf("expected fallback success, got %v", err)
	}
	if resp == nil || resp.Artifact == nil || resp.Artifact.Adapter != "secondary" {
		t.Fatalf("expected secondary adapter response")
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 call reports, got %d", len(reports))
	}
	if !reports[1].FallbackUsed {
		t.Fatalf("expected fallback_used true for fallback call")
	}
}
