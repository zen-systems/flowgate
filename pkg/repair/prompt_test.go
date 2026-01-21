package repair

import (
	"strings"
	"testing"

	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/gate"
)

func TestGenerateEscalationPromptRequestsDiff(t *testing.T) {
	art := artifact.New("original", "mock", "mock-1", "prompt")
	result := gate.NewFailingResult(100, []gate.Violation{
		{
			Rule:    "rule",
			Message: "message",
		},
	}, nil)

	prompt := GenerateEscalationPrompt(art, result, true)
	if !strings.Contains(prompt, "Do NOT repeat the previous output") {
		t.Fatalf("missing repeat warning")
	}
	if !strings.Contains(prompt, "unified diff") {
		t.Fatalf("missing unified diff request")
	}
}
