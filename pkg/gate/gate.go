package gate

import (
	"github.com/zen-systems/flowgate/pkg/artifact"
)

// Gate defines the interface for quality gates.
type Gate interface {
	// Evaluate checks an artifact against quality criteria.
	Evaluate(artifact *artifact.Artifact) (*GateResult, error)

	// Name returns the gate identifier.
	Name() string
}

// GateResult contains the outcome of a gate evaluation.
type GateResult struct {
	Passed      bool                `json:"passed"`
	Score       int                 `json:"score"`
	Violations  []Violation         `json:"violations,omitempty"`
	RepairHints []string            `json:"repair_hints,omitempty"`
	Diagnostics *CommandDiagnostics `json:"diagnostics,omitempty"`
}

// Violation describes a specific quality issue.
type Violation struct {
	Rule       string `json:"rule"`
	Severity   string `json:"severity"` // "error", "warning", "info"
	Message    string `json:"message"`
	Location   string `json:"location,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// NewPassingResult creates a result indicating the gate passed.
func NewPassingResult(score int) *GateResult {
	return &GateResult{
		Passed: true,
		Score:  score,
	}
}

// NewFailingResult creates a result indicating the gate failed.
func NewFailingResult(score int, violations []Violation, hints []string) *GateResult {
	return &GateResult{
		Passed:      false,
		Score:       score,
		Violations:  violations,
		RepairHints: hints,
	}
}
