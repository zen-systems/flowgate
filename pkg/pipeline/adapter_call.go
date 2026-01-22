package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/config"
)

type callTarget struct {
	Adapter string
	Model   string
}

func callAdapterWithPolicy(
	ctx context.Context,
	adapters map[string]adapter.Adapter,
	adapterName string,
	model string,
	prompt string,
	cfg *config.RoutingConfig,
	tracker *costTracker,
) (*adapter.Response, []adapter.CallReport, error) {
	targets := buildTargets(adapterName, model, cfg)
	retryCfg := retrySettings(cfg)
	var reports []adapter.CallReport
	var lastErr error

	for idx, target := range targets {
		adapterImpl, ok := adapters[target.Adapter]
		if !ok {
			return nil, reports, fmt.Errorf("adapter %s not found", target.Adapter)
		}

		for attempt := 0; attempt <= retryCfg.MaxRetries; attempt++ {
			if tracker != nil {
				if err := tracker.checkBudget(target.Adapter, target.Model); err != nil {
					return nil, reports, err
				}
			}

			resp, err := adapterImpl.Generate(ctx, target.Model, prompt)
			if err == nil {
				usage := normalizeUsage(resp.Usage)
				cost, _ := estimateCost(cfgPricing(cfg), target.Adapter, target.Model, usage)
				reports = append(reports, adapter.CallReport{
					Adapter:      target.Adapter,
					Model:        target.Model,
					Usage:        usage,
					Cost:         cost,
					Retries:      attempt,
					FallbackUsed: idx > 0,
				})
				return resp, reports, nil
			}

			lastErr = err
			if !adapter.IsTransient(err) || attempt == retryCfg.MaxRetries {
				reports = append(reports, adapter.CallReport{
					Adapter:      target.Adapter,
					Model:        target.Model,
					Usage:        adapter.Usage{},
					Cost:         adapter.Cost{Currency: "USD"},
					Retries:      attempt,
					FallbackUsed: idx > 0,
					Error:        err.Error(),
				})
				break
			}

			backoff := computeBackoff(retryCfg.BaseBackoffMs, retryCfg.MaxBackoffMs, attempt)
			if err := sleepWithContext(ctx, backoff); err != nil {
				return nil, reports, err
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("adapter call failed")
	}
	return nil, reports, lastErr
}

func buildTargets(adapterName, model string, cfg *config.RoutingConfig) []callTarget {
	targets := []callTarget{{Adapter: adapterName, Model: model}}
	if cfg == nil || !cfg.Fallback.AllowFallback {
		return targets
	}
	chain := resolveFallbackChain(cfg, adapterName, model)
	for _, entry := range chain {
		targets = append(targets, callTarget{Adapter: entry.Adapter, Model: entry.Model})
	}
	return targets
}

func resolveFallbackChain(cfg *config.RoutingConfig, adapterName, model string) []config.RouteTarget {
	if cfg == nil || cfg.Fallback.FallbackChain == nil {
		return nil
	}
	key := fmt.Sprintf("%s/%s", adapterName, model)
	if chain, ok := cfg.Fallback.FallbackChain[key]; ok {
		return chain
	}
	if chain, ok := cfg.Fallback.FallbackChain[adapterName]; ok {
		return chain
	}
	return nil
}

func retrySettings(cfg *config.RoutingConfig) config.RetryConfig {
	if cfg == nil {
		return config.RetryConfig{MaxRetries: 2, BaseBackoffMs: 200, MaxBackoffMs: 2000}
	}
	return cfg.Retry
}

func computeBackoff(baseMs, maxMs, attempt int) time.Duration {
	backoff := time.Duration(baseMs) * time.Millisecond
	for i := 0; i < attempt; i++ {
		backoff *= 2
		if backoff >= time.Duration(maxMs)*time.Millisecond {
			return time.Duration(maxMs) * time.Millisecond
		}
	}
	if backoff > time.Duration(maxMs)*time.Millisecond {
		return time.Duration(maxMs) * time.Millisecond
	}
	return backoff
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func cfgPricing(cfg *config.RoutingConfig) config.PricingConfig {
	if cfg == nil {
		return nil
	}
	return cfg.Pricing
}
