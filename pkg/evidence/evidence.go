package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunRecord captures run-level metadata.
type RunRecord struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	PipelineFile string            `json:"pipeline_file"`
	InputHash    string            `json:"input_hash"`
	Workspace    string            `json:"workspace"`
	ToolVersions map[string]string `json:"tool_versions,omitempty"`
}

// StageRecord captures evidence for a single stage.
type StageRecord struct {
	Name           string            `json:"name"`
	Adapter        string            `json:"adapter"`
	Model          string            `json:"model"`
	Prompt         string            `json:"prompt,omitempty"`
	PromptHash     string            `json:"prompt_hash,omitempty"`
	Output         string            `json:"output,omitempty"`
	OutputHash     string            `json:"output_hash,omitempty"`
	Artifacts      map[string]string `json:"artifacts,omitempty"`
	GateResults    []GateRecord      `json:"gate_results,omitempty"`
	ApplyResult    *ApplyRecord      `json:"apply_result,omitempty"`
	DurationMillis int64             `json:"duration_ms"`
	Attempts       []AttemptRecord   `json:"attempts,omitempty"`
}

// ApplyRecord captures workspace apply behavior.
type ApplyRecord struct {
	AppliedFiles    []string `json:"applied_files,omitempty"`
	DeletedFiles    []string `json:"deleted_files,omitempty"`
	UsedUnifiedDiff bool     `json:"used_unified_diff"`
}

// GateRecord captures gate evaluation results.
type GateRecord struct {
	Name           string          `json:"name"`
	Passed         bool            `json:"passed"`
	Score          int             `json:"score"`
	Violations     []Violation     `json:"violations,omitempty"`
	RepairHints    []string        `json:"repair_hints,omitempty"`
	Kind           string          `json:"kind,omitempty"`
	Diagnostics    json.RawMessage `json:"diagnostics,omitempty"`
	Error          string          `json:"error,omitempty"`
	DurationMillis int64           `json:"duration_ms"`
}

// Violation mirrors gate violation details.
type Violation struct {
	Rule       string `json:"rule"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Location   string `json:"location,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// AttemptRecord captures each attempt to satisfy gates.
type AttemptRecord struct {
	Attempt        int          `json:"attempt"`
	PromptHash     string       `json:"prompt_hash,omitempty"`
	GateResults    []GateRecord `json:"gate_results,omitempty"`
	ApplyError     string       `json:"apply_error,omitempty"`
	Succeeded      bool         `json:"succeeded"`
	DurationMillis int64        `json:"duration_ms"`
}

// Writer writes evidence bundles to disk.
type Writer struct {
	baseDir string
	runDir  string
}

// NewWriter creates a new evidence writer rooted at baseDir/runID.
func NewWriter(baseDir, runID string) (*Writer, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory is required")
	}
	if runID == "" {
		return nil, fmt.Errorf("run ID is required")
	}

	runDir := filepath.Join(baseDir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(runDir, "stages"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(runDir, "gates"), 0755); err != nil {
		return nil, err
	}

	return &Writer{baseDir: baseDir, runDir: runDir}, nil
}

// RunDir returns the run directory path.
func (w *Writer) RunDir() string {
	return w.runDir
}

// WriteRun writes run metadata to run.json.
func (w *Writer) WriteRun(record RunRecord) error {
	return writeJSON(filepath.Join(w.runDir, "run.json"), record)
}

// WriteStage writes a stage record to stages/<stage>.json.
func (w *Writer) WriteStage(record StageRecord) error {
	path := filepath.Join(w.runDir, "stages", fmt.Sprintf("%s.json", record.Name))
	return writeJSON(path, record)
}

// WriteGateLog writes gate logs to gates/<stage>-<gate>.log.
func (w *Writer) WriteGateLog(stageName, gateName, content string) error {
	if stageName == "" || gateName == "" {
		return fmt.Errorf("stage name and gate name are required")
	}
	path := filepath.Join(w.runDir, "gates", fmt.Sprintf("%s-%s.log", stageName, gateName))
	return os.WriteFile(path, []byte(content), 0644)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
