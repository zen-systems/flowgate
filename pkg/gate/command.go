package gate

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"

	"github.com/zen-systems/flowgate/pkg/artifact"
)

// CommandDiagnostics captures execution details for a command gate.
type CommandDiagnostics struct {
	Command  []string      `json:"command"`
	Workdir  string        `json:"workdir,omitempty"`
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
}

// CommandGate executes a local command as a gate.
type CommandGate struct {
	name    string
	command []string
	workdir string
}

// CommandGateConfig defines configuration for a command gate.
type CommandGateConfig struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
	Workdir string   `yaml:"workdir,omitempty"`
}

// NewCommandGate creates a new command gate.
func NewCommandGate(name string, command []string, workdir string) (*CommandGate, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("command gate requires a command")
	}
	if name == "" {
		name = command[0]
	}
	return &CommandGate{name: name, command: command, workdir: workdir}, nil
}

// Name returns the gate identifier.
func (g *CommandGate) Name() string {
	return g.name
}

// Evaluate runs the command and returns a GateResult with diagnostics.
func (g *CommandGate) Evaluate(_ *artifact.Artifact) (*GateResult, error) {
	cmd := exec.Command(g.command[0], g.command[1:]...)
	if g.workdir != "" {
		cmd.Dir = g.workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("command gate failed to run: %w", err)
		}
	}

	passed := exitCode == 0
	result := &GateResult{
		Passed: passed,
		Score:  exitCode,
		Diagnostics: &CommandDiagnostics{
			Command:  append([]string{}, g.command...),
			Workdir:  g.workdir,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
			Duration: duration,
		},
	}

	if !passed {
		result.Violations = []Violation{
			{
				Rule:     "command_failed",
				Severity: "error",
				Message:  fmt.Sprintf("command exited with status %d", exitCode),
			},
		}
		if stderr.Len() > 0 {
			result.RepairHints = []string{"Review stderr output for failure details"}
		}
	}

	return result, nil
}
