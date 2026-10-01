package router

import (
	"sort"
	"strings"

	"github.com/zen-systems/flowgate/pkg/config"
)

// RuleSet contains the compiled routing rules for pattern matching.
type RuleSet struct {
	config *config.RoutingConfig
	// Compiled rules ordered by priority (longer triggers first for specificity)
	rules []compiledRule
}

type compiledRule struct {
	taskType string
	trigger  string
	adapter  string
	model    string
}

// NewRuleSet creates a new rule set from routing configuration.
func NewRuleSet(cfg *config.RoutingConfig) *RuleSet {
	rs := &RuleSet{config: cfg}
	rs.compile()
	return rs
}

// compile builds the list of rules sorted by trigger length (longest first).
func (rs *RuleSet) compile() {
	rs.rules = nil

	for name, taskType := range rs.config.TaskTypes {
		for _, trigger := range taskType.Triggers {
			rs.rules = append(rs.rules, compiledRule{
				taskType: name,
				trigger:  strings.ToLower(trigger),
				adapter:  taskType.Adapter,
				model:    taskType.Model,
			})
		}
	}

	// Sort by trigger length (longer triggers are more specific)
	sort.Slice(rs.rules, func(i, j int) bool {
		return len(rs.rules[i].trigger) > len(rs.rules[j].trigger)
	})
}

// Match finds the best matching rule for a prompt.
// Returns the adapter name and model, or defaults if no match.
func (rs *RuleSet) Match(prompt string) (adapter string, model string) {
	promptLower := strings.ToLower(prompt)

	for _, rule := range rs.rules {
		if containsTrigger(promptLower, rule.trigger) {
			return rule.adapter, rule.model
		}
	}

	return rs.config.Default.Adapter, rs.config.Default.Model
}

// MatchWithTaskType finds the best matching rule and returns task type info.
func (rs *RuleSet) MatchWithTaskType(prompt string) (taskType, adapter, model string) {
	promptLower := strings.ToLower(prompt)

	for _, rule := range rs.rules {
		if containsTrigger(promptLower, rule.trigger) {
			return rule.taskType, rule.adapter, rule.model
		}
	}

	return "default", rs.config.Default.Adapter, rs.config.Default.Model
}

// containsTrigger checks if the prompt contains the trigger phrase.
// It looks for the trigger as a word or phrase boundary match.
func containsTrigger(prompt, trigger string) bool {
	if trigger == "" {
		return false
	}

	start := 0
	for {
		idx := strings.Index(prompt[start:], trigger)
		if idx == -1 {
			return false
		}

		realIdx := start + idx

		// Check boundaries
		isStartBoundary := realIdx == 0 || !isWordChar(prompt[realIdx-1])
		isEndBoundary := (realIdx+len(trigger) == len(prompt)) || !isWordChar(prompt[realIdx+len(trigger)])

		if isStartBoundary && isEndBoundary {
			return true
		}

		// Move forward to search for next occurrence
		start = realIdx + 1
	}
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
