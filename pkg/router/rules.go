package router

import (
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
	for i := 0; i < len(rs.rules); i++ {
		for j := i + 1; j < len(rs.rules); j++ {
			if len(rs.rules[j].trigger) > len(rs.rules[i].trigger) {
				rs.rules[i], rs.rules[j] = rs.rules[j], rs.rules[i]
			}
		}
	}
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
	idx := strings.Index(prompt, trigger)
	if idx == -1 {
		return false
	}

	// Check word boundary before trigger
	if idx > 0 {
		prev := prompt[idx-1]
		if isWordChar(prev) {
			return false
		}
	}

	// Check word boundary after trigger
	endIdx := idx + len(trigger)
	if endIdx < len(prompt) {
		next := prompt[endIdx]
		if isWordChar(next) {
			return false
		}
	}

	return true
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
