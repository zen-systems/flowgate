package gate

import (
	"os/exec"
	"testing"
)

func TestCommandGateCapturesOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	gate, err := NewCommandGate("check", []string{"sh", "-c", "echo hello; echo err 1>&2; exit 1"}, "")
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	result, err := gate.Evaluate(nil)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failure")
	}
	if result.Diagnostics == nil {
		t.Fatalf("expected diagnostics")
	}
	if result.Diagnostics.Stdout == "" || result.Diagnostics.Stderr == "" {
		t.Fatalf("expected stdout and stderr to be captured")
	}
	if result.Diagnostics.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code")
	}
}
