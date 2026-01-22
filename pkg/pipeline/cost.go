package pipeline

import (
	"fmt"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/evidence"
)

type costTracker struct {
	pricing       config.PricingConfig
	totalUsage    adapter.Usage
	totalAmount   float64
	currency      string
	calls         []adapter.CallReport
	maxBudgetUSD  float64
	budgetStatus  *evidence.BudgetStatus
	lastUsageHint *adapter.Usage
}

func newCostTracker(cfg *config.RoutingConfig, maxBudgetUSD float64) *costTracker {
	pricing := config.PricingConfig(nil)
	if cfg != nil {
		pricing = cfg.Pricing
	}
	return &costTracker{
		pricing:      pricing,
		currency:     "USD",
		maxBudgetUSD: maxBudgetUSD,
	}
}

func (t *costTracker) checkBudget(adapterName, model string) error {
	if t == nil || t.maxBudgetUSD <= 0 {
		return nil
	}
	if t.budgetStatus == nil {
		t.budgetStatus = &evidence.BudgetStatus{MaxAmount: t.maxBudgetUSD}
	}
	if t.totalAmount >= t.maxBudgetUSD {
		reason := fmt.Sprintf("budget %.2f exceeded (current total %.2f)", t.maxBudgetUSD, t.totalAmount)
		t.budgetStatus.Exceeded = true
		t.budgetStatus.Reason = reason
		return fmt.Errorf("%s", reason)
	}
	if t.lastUsageHint == nil {
		return nil
	}

	cost, ok := estimateCost(t.pricing, adapterName, model, *t.lastUsageHint)
	if !ok {
		return nil
	}
	projected := t.totalAmount + cost.Amount
	if projected > t.maxBudgetUSD {
		reason := fmt.Sprintf("budget %.2f exceeded (projected total %.2f)", t.maxBudgetUSD, projected)
		t.budgetStatus.Exceeded = true
		t.budgetStatus.Reason = reason
		return fmt.Errorf("%s", reason)
	}
	return nil
}

func (t *costTracker) recordReports(reports []adapter.CallReport) {
	if t == nil {
		return
	}
	for _, report := range reports {
		t.calls = append(t.calls, report)
		if report.Error != "" {
			continue
		}
		t.totalAmount += report.Cost.Amount
		t.totalUsage = addUsage(t.totalUsage, report.Usage)
		t.lastUsageHint = &report.Usage
	}
}

func (t *costTracker) report() *evidence.RunCostReport {
	if t == nil {
		return nil
	}
	if t.budgetStatus == nil && t.maxBudgetUSD > 0 {
		t.budgetStatus = &evidence.BudgetStatus{MaxAmount: t.maxBudgetUSD}
	}
	return &evidence.RunCostReport{
		Currency:    t.currency,
		TotalAmount: t.totalAmount,
		TotalUsage:  t.totalUsage,
		Calls:       t.calls,
		Budget:      t.budgetStatus,
	}
}

func normalizeUsage(u *adapter.Usage) adapter.Usage {
	if u == nil {
		return adapter.Usage{}
	}
	usage := *u
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

func estimateCost(pricing config.PricingConfig, adapterName, model string, usage adapter.Usage) (adapter.Cost, bool) {
	pricingModel := "per_1k_tokens"
	entry, ok := pricingFor(pricing, adapterName, model)
	if !ok {
		return adapter.Cost{Currency: "USD"}, false
	}

	promptCost := (float64(usage.PromptTokens) / 1000.0) * entry.PromptPer1K
	completionCost := (float64(usage.CompletionTokens) / 1000.0) * entry.CompletionPer1K
	return adapter.Cost{
		Currency:     "USD",
		Amount:       promptCost + completionCost,
		IsEstimate:   true,
		PricingModel: pricingModel,
	}, true
}

func pricingFor(pricing config.PricingConfig, adapterName, model string) (config.ModelPricing, bool) {
	if pricing == nil {
		return config.ModelPricing{}, false
	}
	if adapterPricing, ok := pricing[adapterName]; ok {
		if entry, ok := adapterPricing[model]; ok {
			return entry, true
		}
		if entry, ok := adapterPricing["default"]; ok {
			return entry, true
		}
	}
	return config.ModelPricing{}, false
}

func addUsage(a adapter.Usage, b adapter.Usage) adapter.Usage {
	return adapter.Usage{
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		TotalTokens:      a.TotalTokens + b.TotalTokens,
	}
}
