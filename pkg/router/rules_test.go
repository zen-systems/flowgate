package router

import (
	"testing"

	"github.com/zen-systems/flowgate/pkg/config"
)

func TestRuleSet_Match(t *testing.T) {
	cfg := config.DefaultRoutingConfig()
	rs := NewRuleSet(cfg)

	tests := []struct {
		name            string
		prompt          string
		expectedAdapter string
		expectedModel   string
	}{
		{
			name:            "research trigger",
			prompt:          "Research tree-sitter Go bindings",
			expectedAdapter: "google",
			expectedModel:   "gemini-2.0-pro",
		},
		{
			name:            "find trigger",
			prompt:          "Find all usages of this function",
			expectedAdapter: "google",
			expectedModel:   "gemini-2.0-pro",
		},
		{
			name:            "what is trigger",
			prompt:          "What is the difference between channels and mutexes?",
			expectedAdapter: "google",
			expectedModel:   "gemini-2.0-pro",
		},
		{
			name:            "summarize trigger",
			prompt:          "Summarize this document for me",
			expectedAdapter: "openai",
			expectedModel:   "gpt-5.2-thinking",
		},
		{
			name:            "tldr trigger",
			prompt:          "TLDR this article",
			expectedAdapter: "openai",
			expectedModel:   "gpt-5.2-thinking",
		},
		{
			name:            "scaffold trigger",
			prompt:          "Scaffold a REST API with authentication",
			expectedAdapter: "openai",
			expectedModel:   "gpt-5.2-codex",
		},
		{
			name:            "implement trigger",
			prompt:          "Implement a binary search function",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
		{
			name:            "code trigger",
			prompt:          "Code a function to parse JSON",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
		{
			name:            "write a function trigger",
			prompt:          "Write a function that validates email addresses",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
		{
			name:            "refactor trigger",
			prompt:          "Refactor this class to use composition",
			expectedAdapter: "openai",
			expectedModel:   "gpt-5.2-codex",
		},
		{
			name:            "debug trigger",
			prompt:          "Debug this nil pointer exception",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
		{
			name:            "fix trigger",
			prompt:          "Fix the null pointer exception in this code",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
		{
			name:            "review trigger",
			prompt:          "Review this pull request for style issues",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
		{
			name:            "memory leak trigger (complex_debug)",
			prompt:          "Fix the memory leak in this code",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-opus-4-20250514",
		},
		{
			name:            "security trigger",
			prompt:          "Review this for security vulnerabilities",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-opus-4-20250514",
		},
		{
			name:            "architecture trigger",
			prompt:          "Help me design the architecture for a distributed cache",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-opus-4-20250514",
		},
		{
			name:            "math calculate trigger",
			prompt:          "Calculate the complexity of this algorithm",
			expectedAdapter: "openai",
			expectedModel:   "gpt-5.2-pro",
		},
		{
			name:            "default - no trigger match",
			prompt:          "Hello, how are you today?",
			expectedAdapter: "anthropic",
			expectedModel:   "claude-sonnet-4-20250514",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, model := rs.Match(tt.prompt)
			if adapter != tt.expectedAdapter {
				t.Errorf("Match() adapter = %v, want %v", adapter, tt.expectedAdapter)
			}
			if model != tt.expectedModel {
				t.Errorf("Match() model = %v, want %v", model, tt.expectedModel)
			}
		})
	}
}

func TestRuleSet_MatchWithTaskType(t *testing.T) {
	cfg := config.DefaultRoutingConfig()
	rs := NewRuleSet(cfg)

	tests := []struct {
		name             string
		prompt           string
		expectedTaskType string
	}{
		{
			name:             "research task",
			prompt:          "Research best practices for Go error handling",
			expectedTaskType: "research",
		},
		{
			name:             "implement task",
			prompt:          "Implement a rate limiter",
			expectedTaskType: "implement",
		},
		{
			name:             "default task",
			prompt:          "What time is it?",
			expectedTaskType: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskType, _, _ := rs.MatchWithTaskType(tt.prompt)
			if taskType != tt.expectedTaskType {
				t.Errorf("MatchWithTaskType() taskType = %v, want %v", taskType, tt.expectedTaskType)
			}
		})
	}
}

func TestContainsTrigger(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		trigger  string
		expected bool
	}{
		{
			name:     "exact match at start",
			prompt:   "research this topic",
			trigger:  "research",
			expected: true,
		},
		{
			name:     "exact match in middle",
			prompt:   "please research this topic",
			trigger:  "research",
			expected: true,
		},
		{
			name:     "exact match at end",
			prompt:   "do some research",
			trigger:  "research",
			expected: true,
		},
		{
			name:     "case insensitive match",
			prompt:   "RESEARCH this topic",
			trigger:  "research",
			expected: true,
		},
		{
			name:     "partial word - should not match",
			prompt:   "preresearch the topic",
			trigger:  "research",
			expected: false,
		},
		{
			name:     "partial word suffix - should not match",
			prompt:   "researching the topic",
			trigger:  "research",
			expected: false,
		},
		{
			name:     "multi-word trigger",
			prompt:   "write a function to parse JSON",
			trigger:  "write a function",
			expected: true,
		},
		{
			name:     "trigger with punctuation after",
			prompt:   "fix, the bug",
			trigger:  "fix",
			expected: true,
		},
		{
			name:     "no match",
			prompt:   "hello world",
			trigger:  "research",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// containsTrigger expects lowercase inputs
			result := containsTrigger(toLower(tt.prompt), tt.trigger)
			if result != tt.expected {
				t.Errorf("containsTrigger(%q, %q) = %v, want %v",
					tt.prompt, tt.trigger, result, tt.expected)
			}
		})
	}
}

func TestRuleSet_LongerTriggerPrecedence(t *testing.T) {
	// Create a config where "large refactor" should take precedence over "refactor"
	cfg := &config.RoutingConfig{
		TaskTypes: map[string]config.TaskType{
			"simple_refactor": {
				Triggers: []string{"refactor"},
				Adapter:  "openai",
				Model:    "gpt-5.2-instant",
			},
			"large_refactor": {
				Triggers: []string{"large refactor"},
				Adapter:  "openai",
				Model:    "gpt-5.2-codex",
			},
		},
		Default: config.RouteTarget{
			Adapter: "anthropic",
			Model:   "claude-sonnet-4-20250514",
		},
	}

	rs := NewRuleSet(cfg)

	// "large refactor" should match the longer trigger
	adapter, model := rs.Match("Please do a large refactor of this module")
	if adapter != "openai" || model != "gpt-5.2-codex" {
		t.Errorf("Expected large_refactor rule, got adapter=%s model=%s", adapter, model)
	}

	// "refactor" should match the shorter trigger
	adapter, model = rs.Match("Refactor this function")
	if adapter != "openai" || model != "gpt-5.2-instant" {
		t.Errorf("Expected simple_refactor rule, got adapter=%s model=%s", adapter, model)
	}
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
