package gate

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
)

func TestCommandGateCapturesOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	gate, err := NewCommandGate("check", []string{"sh", "-c", "echo hello; echo err 1>&2; exit 1"}, "", nil, false, "", nil, "none", "", true)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	result, err := gate.Evaluate(context.Background(), nil)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failure")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics")
	}

	var diag CommandDiagnostics
	if err := json.Unmarshal(result.Diagnostics, &diag); err != nil {
		t.Fatalf("unmarshal diagnostics: %v", err)
	}
	if diag.Stdout == "" || diag.Stderr == "" {
		t.Fatalf("expected stdout and stderr to be captured")
	}
	if diag.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code")
	}
}

func TestCommandGateAllowedExact(t *testing.T) {
	command := []string{"go", "test", "./..."}
	allowed := []string{"go test", "npm test"}

	if matchesAllowedExact(command, allowed) {
		t.Fatalf("expected command to be denied")
	}

	allowed = []string{"go test ./..."}
	if !matchesAllowedExact(command, allowed) {
		t.Fatalf("expected command to be allowed")
	}

	if matchesAllowedExact([]string{"go", "vet", "./..."}, allowed) {
		t.Fatalf("expected command to be denied")
	}

	allowed = []string{"go test ./..."}
	if matchesAllowedExact([]string{"go", "test", "./...", "-exec", "rm", "-rf", "/"}, allowed) {
		t.Fatalf("expected command with extra args to be denied")
	}
}
