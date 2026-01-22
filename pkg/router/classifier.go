package router

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/config"
)

// Classifier provides smart prompt classification.
type Classifier struct {
	adapters map[string]adapter.Adapter
	config   *config.RoutingConfig
}

// NewClassifier creates a new classifier with adapters and routing config.
func NewClassifier(adapters map[string]adapter.Adapter, cfg *config.RoutingConfig) *Classifier {
	return &Classifier{adapters: adapters, config: cfg}
}

// Classify determines the task type for a prompt.
func (c *Classifier) Classify(ctx context.Context, prompt string) (*Decision, error) {
	decision := HeuristicDecision(prompt, c.config)
	if decision == nil {
		decision = &Decision{TaskType: "default", Confidence: 0}
	}

	threshold := classifierThreshold(c.config)
	if !shouldUseLLMTieBreaker(c.config, decision, threshold) {
		return decision, nil
	}

	adapterName := strings.TrimSpace(c.config.ClassifierAdapter)
	model := strings.TrimSpace(c.config.ClassifierModel)
	if adapterName == "" || model == "" {
		return decision, nil
	}

	adapterImpl, ok := c.adapters[adapterName]
	if !ok || adapterImpl == nil {
		return decision, nil
	}

	promptText := buildClassifierPrompt(prompt, decision.Candidates)
	resp, err := adapterImpl.Generate(ctx, model, promptText)
	if err != nil {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("classifier error: %v", err))
		return decision, err
	}
	if resp == nil || resp.Artifact == nil {
		decision.Reasons = append(decision.Reasons, "classifier returned empty response")
		return decision, fmt.Errorf("classifier returned empty response")
	}

	picked, err := parseClassifierResponse(resp.Artifact.Content)
	if err != nil {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("classifier response invalid: %v", err))
		return decision, err
	}

	if !validTaskType(picked.TaskType, decision.Candidates) {
		decision.Reasons = append(decision.Reasons, "classifier task_type not in candidates")
		return decision, fmt.Errorf("classifier task_type not in candidates")
	}
	if picked.Confidence < 0 || picked.Confidence > 1 {
		decision.Reasons = append(decision.Reasons, "classifier confidence out of range")
		return decision, fmt.Errorf("classifier confidence out of range")
	}

	decision.TaskType = picked.TaskType
	decision.Confidence = picked.Confidence
	decision.UsedLLM = true
	decision.ClassifierAdapter = adapterName
	decision.ClassifierModel = model
	decision.Reasons = append(decision.Reasons, picked.Reason)

	return decision, nil
}

type classifierPick struct {
	TaskType   string  `json:"task_type"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func parseClassifierResponse(content string) (*classifierPick, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var pick classifierPick
	if err := json.Unmarshal([]byte(content), &pick); err != nil {
		return nil, err
	}
	if pick.TaskType == "" {
		return nil, fmt.Errorf("missing task_type")
	}
	return &pick, nil
}

func validTaskType(taskType string, candidates []Candidate) bool {
	if taskType == "default" {
		return true
	}
	for _, candidate := range candidates {
		if candidate.TaskType == taskType {
			return true
		}
	}
	return false
}

func buildClassifierPrompt(userPrompt string, candidates []Candidate) string {
	var sb strings.Builder
	sb.WriteString("You are a routing classifier. Choose the best task_type.\n")
	sb.WriteString("Return ONLY JSON: {\"task_type\":\"...\",\"confidence\":0-1,\"reason\":\"...\"}.\n\n")
	sb.WriteString("User prompt:\n")
	sb.WriteString(userPrompt)
	sb.WriteString("\n\nCandidates:\n")

	for _, c := range candidates {
		sb.WriteString(fmt.Sprintf("- %s (score=%d, adapter=%s, model=%s)\n", c.TaskType, c.Score, c.Adapter, c.Model))
		if len(c.Triggers) > 0 {
			sb.WriteString(fmt.Sprintf("  triggers: %s\n", strings.Join(c.Triggers, ", ")))
		}
	}

	return sb.String()
}

func shouldUseLLMTieBreaker(cfg *config.RoutingConfig, decision *Decision, threshold float64) bool {
	if cfg == nil || decision == nil {
		return false
	}
	if cfg.EnableLLMTieBreaker != nil && !*cfg.EnableLLMTieBreaker {
		return false
	}
	if decision.Confidence >= threshold {
		return false
	}
	if len(decision.Candidates) <= 1 {
		return false
	}
	return true
}

func classifierThreshold(cfg *config.RoutingConfig) float64 {
	if cfg == nil || cfg.ClassifierConfidenceThreshold <= 0 {
		return 0.65
	}
	return cfg.ClassifierConfidenceThreshold
}

// HeuristicDecision scores task types using trigger matches.
func HeuristicDecision(prompt string, cfg *config.RoutingConfig) *Decision {
	if cfg == nil {
		return &Decision{TaskType: "default", Confidence: 0}
	}
	promptLower := strings.ToLower(prompt)

	var candidates []Candidate
	for taskType, task := range cfg.TaskTypes {
		var matched []string
		for _, trig := range task.Triggers {
			trigger := strings.ToLower(trig)
			if containsTrigger(promptLower, trigger) {
				matched = append(matched, trig)
			}
		}
		if len(matched) == 0 {
			continue
		}
		candidates = append(candidates, Candidate{
			TaskType: taskType,
			Score:    len(matched),
			Triggers: matched,
			Adapter:  task.Adapter,
			Model:    task.Model,
		})
	}

	if len(candidates) == 0 {
		return &Decision{
			TaskType:   "default",
			Confidence: 0,
			Reasons:    []string{"no triggers matched; using default"},
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].TaskType < candidates[j].TaskType
		}
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > 3 {
		candidates = candidates[:3]
	}

	topScore := candidates[0].Score
	secondScore := 0
	if len(candidates) > 1 {
		secondScore = candidates[1].Score
	}

	margin := float64(topScore-secondScore) / float64(maxInt(topScore, 1))
	strength := float64(minInt(topScore, 5)) / 5.0
	confidence := 0.75*margin + 0.25*strength
	if topScore >= 2 && secondScore == 0 {
		confidence = maxFloat(confidence, 0.9)
	}
	if topScore >= 3 {
		confidence = minFloat(confidence+0.15, 1.0)
	}

	reasons := []string{fmt.Sprintf("top_score=%d second_score=%d", topScore, secondScore)}

	return &Decision{
		TaskType:   candidates[0].TaskType,
		Confidence: confidence,
		Reasons:    reasons,
		Candidates: candidates,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
