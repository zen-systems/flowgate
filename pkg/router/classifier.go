package router

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zen-systems/flowgate/pkg/config"
	"github.com/zen-systems/flowgate/pkg/epistemic"
)

// Classifier provides heuristic prompt classification.
type Classifier struct {
	config *config.RoutingConfig
}

// NewClassifier creates a new classifier with routing config.
func NewClassifier(cfg *config.RoutingConfig) *Classifier {
	return &Classifier{config: cfg}
}

// Classify determines the task type for a prompt.
func (c *Classifier) Classify(_ context.Context, prompt string) (*epistemic.Decision, error) {
	return HeuristicDecision(prompt, c.config), nil
}

// HeuristicDecision scores task types using trigger matches.
func HeuristicDecision(prompt string, cfg *config.RoutingConfig) *epistemic.Decision {
	decision := &epistemic.Decision{
		Kind:       "route",
		Impact:     "med",
		Confidence: 0,
	}
	if cfg == nil {
		decision.Choice = "default"
		decision.Reasons = []string{"routing config missing"}
		decision.NeedsEgress = true
		return decision
	}

	promptLower := strings.ToLower(prompt)
	var scored []scoredCandidate

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
		scored = append(scored, scoredCandidate{taskType: taskType, score: len(matched), triggers: matched})
	}

	if len(scored) == 0 {
		decision.Choice = "default"
		decision.Reasons = []string{"no triggers matched; using default"}
		decision.NeedsEgress = true
		return decision
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].taskType < scored[j].taskType
		}
		return scored[i].score > scored[j].score
	})

	for _, candidate := range scored {
		decision.Candidates = append(decision.Candidates, candidate.taskType)
	}

	topScore := scored[0].score
	secondScore := 0
	if len(scored) > 1 {
		secondScore = scored[1].score
	}

	confidence := float64(topScore-secondScore) / float64(maxInt(topScore, 1))
	if topScore >= 2 && secondScore == 0 {
		confidence = 0.9
	}
	if topScore >= 3 {
		confidence = minFloat(confidence+0.2, 1.0)
	}

	decision.Choice = scored[0].taskType
	decision.Confidence = confidence
	decision.Reasons = []string{fmt.Sprintf("top_score=%d second_score=%d", topScore, secondScore)}
	decision.NeedsEgress = confidence < 0.9 || len(decision.Candidates) > 1

	return decision
}

type scoredCandidate struct {
	taskType string
	score    int
	triggers []string
}

func maxInt(a, b int) int {
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
