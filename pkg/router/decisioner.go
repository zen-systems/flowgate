package router

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/epistemic"
)

// WebSearch fetches external context.
type WebSearch interface {
	Search(ctx context.Context, query string, maxResults int) ([]epistemic.EvidenceRef, error)
}

// Decide builds a routing decision with optional escalation.
func Decide(
	ctx context.Context,
	prompt string,
	adapters map[string]adapter.Adapter,
	cfg *config.RoutingConfig,
	web WebSearch,
) (*epistemic.RoutingDecision, error) {
	if cfg == nil {
		heuristic := HeuristicDecision(prompt, cfg)
		return &epistemic.RoutingDecision{Heuristic: *heuristic, Final: *heuristic}, nil
	}

	heuristic := HeuristicDecision(prompt, cfg)
	final := *heuristic

	policy := cfg.Policy
	shouldEscalate := policy.ShouldEscalate(*heuristic)
	if !shouldEscalate {
		return &epistemic.RoutingDecision{Heuristic: *heuristic, Final: final}, nil
	}

	var evidence []epistemic.EvidenceRef
	if cfg.EnableTavily && web != nil {
		refs, err := web.Search(ctx, prompt, 3)
		if err == nil {
			evidence = append(evidence, refs...)
		}
	}

	adapterName := strings.TrimSpace(cfg.ClassifierAdapter)
	model := strings.TrimSpace(cfg.ClassifierModel)
	adapterImpl, ok := adapters[adapterName]
	if !ok || adapterName == "" || model == "" {
		final.Reasons = append(final.Reasons, "classifier adapter/model not configured")
		return &epistemic.RoutingDecision{Heuristic: *heuristic, Final: final}, nil
	}

	pick, err := runTieBreaker(ctx, adapterImpl, model, prompt, heuristic, cfg, evidence)
	if err != nil {
		final.Reasons = append(final.Reasons, fmt.Sprintf("tie-breaker error: %v", err))
		return &epistemic.RoutingDecision{Heuristic: *heuristic, Final: final}, nil
	}
	if !containsCandidate(heuristic.Candidates, pick.TaskType) {
		final.Reasons = append(final.Reasons, "tie-breaker task_type invalid")
		return &epistemic.RoutingDecision{Heuristic: *heuristic, Final: final}, nil
	}

	final.Choice = pick.TaskType
	final.Confidence = pick.Confidence
	final.Reasons = append(final.Reasons, pick.Reason)
	final.Escalated = true
	final.EvidenceUsed = append(final.EvidenceUsed, evidence...)

	return &epistemic.RoutingDecision{Heuristic: *heuristic, Final: final}, nil
}

type tieBreakerPick struct {
	TaskType   string  `json:"task_type"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func runTieBreaker(
	ctx context.Context,
	adapterImpl adapter.Adapter,
	model string,
	prompt string,
	heuristic *epistemic.Decision,
	cfg *config.RoutingConfig,
	evidence []epistemic.EvidenceRef,
) (*tieBreakerPick, error) {
	request := buildTieBreakerPrompt(prompt, heuristic, cfg, evidence)
	resp, err := adapterImpl.Generate(ctx, model, request)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Artifact == nil {
		return nil, fmt.Errorf("classifier returned empty response")
	}

	pick, err := parseTieBreakerResponse(resp.Artifact.Content)
	if err != nil {
		return nil, err
	}
	if pick.TaskType == "" {
		return nil, fmt.Errorf("missing task_type")
	}
	if pick.Confidence < 0 || pick.Confidence > 1 {
		return nil, fmt.Errorf("confidence out of range")
	}

	return pick, nil
}

func parseTieBreakerResponse(content string) (*tieBreakerPick, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var pick tieBreakerPick
	if err := json.Unmarshal([]byte(content), &pick); err != nil {
		return nil, err
	}

	return &pick, nil
}

func buildTieBreakerPrompt(prompt string, heuristic *epistemic.Decision, cfg *config.RoutingConfig, evidence []epistemic.EvidenceRef) string {
	var sb strings.Builder
	sb.WriteString("You are a routing classifier. Choose the best task_type.\n")
	sb.WriteString("Return ONLY JSON: {\"task_type\":\"...\",\"confidence\":0-1,\"reason\":\"...\"}.\n\n")
	sb.WriteString("User prompt:\n")
	sb.WriteString(prompt)
	sb.WriteString("\n\nCandidates:\n")

	for _, candidate := range heuristic.Candidates {
		if task, ok := cfg.TaskTypes[candidate]; ok {
			sb.WriteString(fmt.Sprintf("- %s => %s/%s\n", candidate, task.Adapter, task.Model))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", candidate))
		}
	}

	if len(evidence) > 0 {
		sb.WriteString("\nExternal context:\n")
		for _, ref := range evidence {
			if ref.Title != "" {
				sb.WriteString(fmt.Sprintf("- %s (%s)\n", ref.Title, ref.URL))
			} else if ref.URL != "" {
				sb.WriteString(fmt.Sprintf("- %s\n", ref.URL))
			}
			if ref.Snippet != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", ref.Snippet))
			}
		}
	}

	return sb.String()
}

func containsCandidate(candidates []string, value string) bool {
	for _, candidate := range candidates {
		if candidate == value {
			return true
		}
	}
	return false
}
