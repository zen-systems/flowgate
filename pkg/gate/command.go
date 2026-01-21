package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/zen-systems/flowgate/pkg/artifact"
)

// CommandDiagnostics captures execution details for a command gate.
type CommandDiagnostics struct {
	Command        []string `json:"command"`
	Workdir        string   `json:"workdir,omitempty"`
	Stdout         string   `json:"stdout,omitempty"`
	Stderr         string   `json:"stderr,omitempty"`
	ExitCode       int      `json:"exit_code"`
	DurationMillis int64    `json:"duration_ms"`
	Error          string   `json:"error,omitempty"`
	BlockedReason  string   `json:"blocked_reason,omitempty"`
	PolicyMode     string   `json:"policy_mode,omitempty"`
	Capability     string   `json:"capability,omitempty"`
}

// CommandGate executes a local command as a gate.
type CommandGate struct {
	name          string
	command       []string
	workdir       string
	allowed       []string
	denyShell     bool
	workspaceRoot string
	templates     []CommandTemplate
	policyMode    string
	capability    string
	shellApproved bool
}

// CommandGateConfig defines configuration for a command gate.
type CommandGateConfig struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
	Workdir string   `yaml:"workdir,omitempty"`
}

// NewCommandGate creates a new command gate.
func NewCommandGate(name string, command []string, workdir string, allowed []string, denyShell bool, workspaceRoot string, templates []CommandTemplate, policyMode string, capability string, shellApproved bool) (*CommandGate, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("command gate requires a command")
	}
	if name == "" {
		name = command[0]
	}
	return &CommandGate{
		name:          name,
		command:       command,
		workdir:       workdir,
		allowed:       allowed,
		denyShell:     denyShell,
		workspaceRoot: workspaceRoot,
		templates:     templates,
		policyMode:    policyMode,
		capability:    capability,
		shellApproved: shellApproved,
	}, nil
}

// Name returns the gate identifier.
func (g *CommandGate) Name() string {
	return g.name
}

// Evaluate runs the command and returns a GateResult with diagnostics.
func (g *CommandGate) Evaluate(ctx context.Context, _ *artifact.Artifact) (*GateResult, error) {
	if blockedReason := g.policyBlockReason(); blockedReason != "" {
		result := g.resultFromDiagnostics(CommandDiagnostics{
			Command:       append([]string{}, g.command...),
			Workdir:       g.workdir,
			ExitCode:      1,
			BlockedReason: blockedReason,
			PolicyMode:    g.policyMode,
			Capability:    g.capability,
		}, false)
		result.Violations = []Violation{
			{
				Rule:     "command_blocked",
				Severity: "error",
				Message:  blockedReason,
			},
		}
		return result, nil
	}

	cmd := exec.CommandContext(ctx, g.command[0], g.command[1:]...)
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
			result := g.resultFromDiagnostics(CommandDiagnostics{
				Command:        append([]string{}, g.command...),
				Workdir:        g.workdir,
				Stdout:         stdout.String(),
				Stderr:         stderr.String(),
				ExitCode:       exitCode,
				DurationMillis: duration.Milliseconds(),
				Error:          err.Error(),
				PolicyMode:     g.policyMode,
				Capability:     g.capability,
			}, false)
			result.Violations = []Violation{
				{
					Rule:     "command_failed",
					Severity: "error",
					Message:  "command failed to start",
				},
			}
			return result, nil
		}
	}

	passed := exitCode == 0
	result := g.resultFromDiagnostics(CommandDiagnostics{
		Command:        append([]string{}, g.command...),
		Workdir:        g.workdir,
		Stdout:         stdout.String(),
		Stderr:         stderr.String(),
		ExitCode:       exitCode,
		DurationMillis: duration.Milliseconds(),
		PolicyMode:     g.policyMode,
		Capability:     g.capability,
	}, passed)

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

func (g *CommandGate) resultFromDiagnostics(diag CommandDiagnostics, passed bool) *GateResult {
	payload, _ := json.Marshal(diag)
	score := 0
	if !passed {
		score = 100
	}
	return &GateResult{
		Passed:      passed,
		Score:       score,
		Kind:        "command",
		Diagnostics: payload,
	}
}

func (g *CommandGate) policyBlockReason() string {
	if !g.denyShell && !g.shellApproved {
		return "shell execution requires explicit approval"
	}
	if g.denyShell && isShellCommand(g.command) {
		return "shell execution is denied by policy"
	}
	if len(g.templates) > 0 {
		ok, reason := matchTemplates(g.command, g.templates, g.workspaceRoot, g.workdir)
		if !ok {
			return reason
		}
		return ""
	}
	if len(g.allowed) > 0 && !matchesAllowedExact(g.command, g.allowed) {
		return "command does not match any allowed template"
	}
	return ""
}

func isShellCommand(command []string) bool {
	if len(command) < 2 {
		return false
	}
	switch command[0] {
	case "sh", "bash", "zsh":
		return command[1] == "-c"
	default:
		return false
	}
}

func matchesAllowedExact(command []string, allowed []string) bool {
	if len(command) == 0 {
		return false
	}
	cmdString := strings.TrimSpace(strings.Join(command, " "))
	for _, prefix := range allowed {
		allowedEntry := strings.TrimSpace(prefix)
		if allowedEntry == "" {
			continue
		}
		if cmdString == allowedEntry {
			return true
		}
	}
	return false
}
