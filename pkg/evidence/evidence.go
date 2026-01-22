package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/router"
)

// RunRecord captures run-level metadata.
type RunRecord struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	PipelineFile    string            `json:"pipeline_file"`
	InputHash       string            `json:"input_hash"`
	Workspace       string            `json:"workspace"`
	ToolVersions    map[string]string `json:"tool_versions,omitempty"`
	CostReport      *RunCostReport    `json:"cost_report,omitempty"`
	RoutingDecision *router.Decision  `json:"routing_decision,omitempty"`
}

// RunCostReport captures aggregated cost/usage information.
type RunCostReport struct {
	Currency    string               `json:"currency"`
	TotalAmount float64              `json:"total_amount"`
	TotalUsage  adapter.Usage        `json:"total_usage"`
	Calls       []adapter.CallReport `json:"calls,omitempty"`
	Budget      *BudgetStatus        `json:"budget,omitempty"`
}

// BudgetStatus captures budget enforcement state.
type BudgetStatus struct {
	MaxAmount float64 `json:"max_amount"`
	Exceeded  bool    `json:"exceeded"`
	Reason    string  `json:"reason,omitempty"`
}

// StageRecord captures evidence for a single stage.
type StageRecord struct {
	Name           string            `json:"name"`
	Adapter        string            `json:"adapter"`
	Model          string            `json:"model"`
	Prompt         string            `json:"prompt,omitempty"`
	PromptRef      string            `json:"prompt_ref,omitempty"`
	PromptHash     string            `json:"prompt_hash,omitempty"`
	PromptLen      int               `json:"prompt_len,omitempty"`
	Output         string            `json:"output,omitempty"`
	OutputRef      string            `json:"output_ref,omitempty"`
	OutputHash     string            `json:"output_hash,omitempty"`
	OutputLen      int               `json:"output_len,omitempty"`
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
	PromptRef      string       `json:"prompt_ref,omitempty"`
	OutputRef      string       `json:"output_ref,omitempty"`
	OutputHash     string       `json:"output_hash,omitempty"`
	OutputLen      int          `json:"output_len,omitempty"`
	WorkspaceUsed  string       `json:"workspace_used,omitempty"`
	WorkspaceMode  string       `json:"workspace_mode,omitempty"`
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
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(runDir, "stages"), 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(runDir, "gates"), 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(runDir, "blobs"), 0700); err != nil {
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
	return os.WriteFile(path, []byte(content), 0600)
}

// WriteBlob stores content in the blob store and returns a relative reference.
func (w *Writer) WriteBlob(kind string, content []byte) (ref string, sha string, err error) {
	sanitized := sanitizeKind(kind)
	sum := sha256.Sum256(content)
	sha = hex.EncodeToString(sum[:])
	filename := fmt.Sprintf("%s-%s.txt", sanitized, sha)
	ref = filepath.ToSlash(filepath.Join("blobs", filename))
	path := filepath.Join(w.runDir, ref)

	if _, err := os.Stat(path); err == nil {
		return ref, sha, nil
	} else if !os.IsNotExist(err) {
		return "", "", err
	}

	if err := os.WriteFile(path, content, 0600); err != nil {
		return "", "", err
	}

	return ref, sha, nil
}

func sanitizeKind(kind string) string {
	if kind == "" {
		return "blob"
	}

	var sb strings.Builder
	for _, r := range strings.ToLower(kind) {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r)
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == '_' || r == '-':
			sb.WriteRune(r)
		}
	}

	if sb.Len() == 0 {
		return "blob"
	}
	return sb.String()
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
