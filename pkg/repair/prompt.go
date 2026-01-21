package repair

import (
	"fmt"
	"strings"

	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/gate"
)

// GenerateRepairPrompt creates a prompt to fix gate failures.
// Phase 2: Implements sophisticated prompt generation.
func GenerateRepairPrompt(original *artifact.Artifact, result *gate.GateResult) string {
	var sb strings.Builder

	sb.WriteString("The following output failed quality checks:\n\n")
	sb.WriteString("---\n")
	sb.WriteString(original.Content)
	sb.WriteString("\n---\n\n")

	sb.WriteString("Issues found:\n")
	for _, v := range result.Violations {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", v.Severity, v.Rule, v.Message))
		if v.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("  Suggestion: %s\n", v.Suggestion))
		}
	}

	if len(result.RepairHints) > 0 {
		sb.WriteString("\nRepair hints:\n")
		for _, hint := range result.RepairHints {
			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
	}

	sb.WriteString("\nPlease fix all issues and provide the corrected output.")

	return sb.String()
}

// GenerateEscalationPrompt creates a stronger prompt when the repair loop is stuck.
func GenerateEscalationPrompt(original *artifact.Artifact, result *gate.GateResult, requireDiff bool) string {
	var sb strings.Builder

	sb.WriteString("The previous outputs are repeating and failed quality checks.\n")
	sb.WriteString("Do NOT repeat the previous output; change the implementation.\n\n")

	sb.WriteString("Issues found:\n")
	for _, v := range result.Violations {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", v.Rule, v.Message))
	}

	if requireDiff {
		sb.WriteString("\nReturn a unified diff only.\n")
	}

	sb.WriteString("\nPrevious output:\n---\n")
	sb.WriteString(original.Content)
	sb.WriteString("\n---\n")
	sb.WriteString("\nProvide a corrected implementation that addresses the issues above.\n")

	return sb.String()
}
