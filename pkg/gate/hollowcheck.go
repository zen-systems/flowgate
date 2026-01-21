package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zen-systems/flowgate/pkg/artifact"
)

// HollowCheckGate wraps the hollowcheck CLI as a quality gate.
type HollowCheckGate struct {
	binaryPath   string
	contractPath string
}

// HollowCheckConfig holds configuration for the hollowcheck gate.
type HollowCheckConfig struct {
	BinaryPath   string `yaml:"binary_path"`
	ContractPath string `yaml:"contract_path"`
}

// hollowcheckOutput represents the JSON output from hollowcheck CLI.
type hollowcheckOutput struct {
	Version      string                 `json:"version"`
	Path         string                 `json:"path"`
	Contract     string                 `json:"contract"`
	Score        int                    `json:"score"`
	Grade        string                 `json:"grade"`
	Threshold    int                    `json:"threshold"`
	Passed       bool                   `json:"passed"`
	FilesScanned int                    `json:"files_scanned"`
	Violations   []hollowcheckIssue     `json:"violations"`
	Breakdown    []hollowcheckBreakdown `json:"breakdown"`
}

// hollowcheckIssue represents a single issue from hollowcheck.
type hollowcheckIssue struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// hollowcheckBreakdown contains per-rule scoring information.
type hollowcheckBreakdown struct {
	Rule       string `json:"rule"`
	Points     int    `json:"points"`
	Violations int    `json:"violations"`
}

// NewHollowCheckGate creates a new hollowcheck gate.
func NewHollowCheckGate(binaryPath, contractPath string) *HollowCheckGate {
	if binaryPath == "" {
		binaryPath = "hollowcheck" // Use PATH lookup
	}
	return &HollowCheckGate{
		binaryPath:   binaryPath,
		contractPath: contractPath,
	}
}

// NewHollowCheckGateFromConfig creates a gate from configuration.
func NewHollowCheckGateFromConfig(cfg HollowCheckConfig) *HollowCheckGate {
	return NewHollowCheckGate(cfg.BinaryPath, cfg.ContractPath)
}

// Name returns the gate identifier.
func (g *HollowCheckGate) Name() string {
	return "hollowcheck"
}

// Evaluate runs hollowcheck on the artifact content.
func (g *HollowCheckGate) Evaluate(ctx context.Context, a *artifact.Artifact) (*GateResult, error) {
	tempDir, err := os.MkdirTemp("", "flowgate-hollowcheck-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := g.writeArtifact(tempDir, a); err != nil {
		return nil, fmt.Errorf("failed to write artifact: %w", err)
	}

	output, err := g.runHollowcheck(ctx, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to run hollowcheck: %w", err)
	}

	result := g.toGateResult(output)
	payload, _ := json.Marshal(output)
	result.Kind = "hollowcheck"
	result.Diagnostics = payload
	return result, nil
}

// writeArtifact writes the artifact content to the temp directory.
// It detects if the content contains multiple files (via markers) or is a single file.
func (g *HollowCheckGate) writeArtifact(dir string, a *artifact.Artifact) error {
	content := a.Content

	if files := parseMultiFileContent(content); len(files) > 0 {
		for path, fileContent := range files {
			fullPath := filepath.Join(dir, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(fullPath, []byte(fileContent), 0644); err != nil {
				return err
			}
		}
		return nil
	}

	ext := ".go"
	if a.Metadata != nil {
		if e, ok := a.Metadata["extension"]; ok {
			ext = e
		}
	}

	filename := "artifact" + ext
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}

// parseMultiFileContent extracts multiple files from content with markers.
// Supports markers like "// file: path/to/file.go" or "# file: script.py"
func parseMultiFileContent(content string) map[string]string {
	files := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentFile string
	var currentContent strings.Builder

	for _, line := range lines {
		if path := extractFilePath(line); path != "" {
			if currentFile != "" {
				files[currentFile] = strings.TrimSuffix(currentContent.String(), "\n")
			}
			currentFile = path
			currentContent.Reset()
			continue
		}

		if currentFile != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	if currentFile != "" {
		files[currentFile] = strings.TrimSuffix(currentContent.String(), "\n")
	}

	return files
}

// extractFilePath extracts a file path from a marker line.
func extractFilePath(line string) string {
	line = strings.TrimSpace(line)

	prefixes := []string{
		"// file:",
		"// File:",
		"# file:",
		"# File:",
		"/* file:",
		"<!-- file:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			path = strings.TrimSuffix(path, "*/")
			path = strings.TrimSuffix(path, "-->")
			return strings.TrimSpace(path)
		}
	}

	return ""
}

// runHollowcheck executes the hollowcheck CLI and returns parsed output.
func (g *HollowCheckGate) runHollowcheck(ctx context.Context, dir string) (*hollowcheckOutput, error) {
	args := []string{"lint", dir, "--format", "json"}

	if g.contractPath != "" {
		args = append(args, "--contract", g.contractPath)
	}

	cmd := exec.CommandContext(ctx, g.binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var output hollowcheckOutput
	if stdout.Len() > 0 {
		if parseErr := json.Unmarshal(stdout.Bytes(), &output); parseErr != nil {
			if err != nil {
				return nil, fmt.Errorf("hollowcheck failed: %v, stderr: %s", err, stderr.String())
			}
			return nil, fmt.Errorf("failed to parse hollowcheck output: %w", parseErr)
		}
		return &output, nil
	}

	if err != nil {
		return nil, fmt.Errorf("hollowcheck failed: %v, stderr: %s", err, stderr.String())
	}

	return &hollowcheckOutput{Passed: true, Score: 0}, nil
}

// toGateResult converts hollowcheck output to a GateResult.
func (g *HollowCheckGate) toGateResult(output *hollowcheckOutput) *GateResult {
	violations := make([]Violation, 0, len(output.Violations))
	repairHints := make([]string, 0)

	for _, issue := range output.Violations {
		location := issue.File
		if issue.Line > 0 {
			location = fmt.Sprintf("%s:%d", issue.File, issue.Line)
		}

		violations = append(violations, Violation{
			Rule:     issue.Rule,
			Severity: issue.Severity,
			Message:  issue.Message,
			Location: location,
		})

		hint := generateRepairHint(issue)
		if hint != "" {
			repairHints = append(repairHints, hint)
		}
	}

	if output.Passed {
		return NewPassingResult(output.Score)
	}

	return NewFailingResult(output.Score, violations, repairHints)
}

// generateRepairHint creates an actionable repair hint from an issue.
func generateRepairHint(issue hollowcheckIssue) string {
	location := issue.File
	if issue.Line > 0 {
		location = fmt.Sprintf("%s:%d", issue.File, issue.Line)
	}

	ruleLower := strings.ToLower(issue.Rule)
	msgLower := strings.ToLower(issue.Message)

	switch {
	case ruleLower == "forbidden_pattern":
		if strings.Contains(msgLower, "todo") {
			return fmt.Sprintf("Remove TODO comment at %s", location)
		}
		if strings.Contains(msgLower, "fixme") {
			return fmt.Sprintf("Address FIXME comment at %s", location)
		}
		if strings.Contains(msgLower, "panic") && strings.Contains(msgLower, "not implemented") {
			return fmt.Sprintf("Replace panic(\"not implemented\") with real implementation at %s", location)
		}
		if strings.Contains(msgLower, "panic") {
			return fmt.Sprintf("Replace panic with proper error handling at %s", location)
		}
		return fmt.Sprintf("Remove forbidden pattern at %s: %s", location, issue.Message)

	case strings.Contains(ruleLower, "stub"), strings.Contains(ruleLower, "low_complexity"):
		return fmt.Sprintf("Implement stub function at %s", location)

	case strings.Contains(ruleLower, "placeholder"), strings.Contains(ruleLower, "mock_data"):
		return fmt.Sprintf("Replace placeholder/mock data at %s", location)

	case strings.Contains(ruleLower, "missing_file"):
		return fmt.Sprintf("Create required file: %s", issue.Message)

	case strings.Contains(ruleLower, "missing_symbol"):
		return fmt.Sprintf("Implement required symbol: %s", issue.Message)

	case strings.Contains(ruleLower, "missing_test"):
		return fmt.Sprintf("Add required test: %s", issue.Message)

	case strings.Contains(ruleLower, "empty"):
		return fmt.Sprintf("Add implementation to empty block at %s", location)

	case strings.Contains(ruleLower, "error"):
		if strings.Contains(msgLower, "ignored") {
			return fmt.Sprintf("Handle error properly at %s", location)
		}
		return fmt.Sprintf("Fix error at %s: %s", location, issue.Message)

	default:
		return fmt.Sprintf("Fix %s violation at %s: %s", issue.Rule, location, issue.Message)
	}
}
