// Package vtp provides VTP adapters for flowgate gates.
package vtp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/gate"
	"github.com/zen-systems/vtp"
)

// HollowcheckAdapter wraps the existing HollowCheckGate to provide VTP interface.
type HollowcheckAdapter struct {
	gate         *gate.HollowCheckGate
	binaryPath   string
	contractPath string
	mu           sync.RWMutex
	pendingEvals map[string]*pendingHollowEval
}

type pendingHollowEval struct {
	result  *gate.GateResult
	created time.Time
}

// NewHollowcheckAdapter creates a new VTP adapter for hollowcheck.
func NewHollowcheckAdapter(binaryPath, contractPath string) *HollowcheckAdapter {
	return &HollowcheckAdapter{
		gate:         gate.NewHollowCheckGate(binaryPath, contractPath),
		binaryPath:   binaryPath,
		contractPath: contractPath,
		pendingEvals: make(map[string]*pendingHollowEval),
	}
}

// Info returns gate metadata.
func (a *HollowcheckAdapter) Info() vtp.GateInfo {
	return vtp.GateInfo{
		ID:          "hollowcheck",
		Name:        "Hollowcheck Gate",
		Version:     "0.1.0",
		Layer:       vtp.LayerStructural,
		Description: "Validates code completeness and quality",
		Capabilities: vtp.GateCapabilities{
			SupportsTwoPhase:    true,
			SupportsRepairHints: true,
			RequiresFilesystem:  true,
			Parallelizable:      false,
		},
	}
}

// Evaluate runs hollowcheck on the artifact.
func (a *HollowcheckAdapter) Evaluate(ctx context.Context, req *vtp.EvaluateRequest) (*vtp.EvaluateResponse, error) {
	// Create artifact for gate evaluation
	art := &artifact.Artifact{
		Content:  req.Artifact.Content,
		Metadata: req.Artifact.Metadata,
	}

	// Run existing gate
	result, err := a.gate.Evaluate(ctx, art)
	if err != nil {
		return nil, err
	}

	// Store for commit
	evalID := vtp.GenerateEvalID()
	a.mu.Lock()
	a.pendingEvals[evalID] = &pendingHollowEval{
		result:  result,
		created: time.Now(),
	}
	a.mu.Unlock()

	// Convert to VTP format
	outcome := vtp.DecisionPass
	if !result.Passed {
		outcome = vtp.DecisionFail
	}

	// Convert violations
	var violations []vtp.Violation
	for _, v := range result.Violations {
		violations = append(violations, vtp.Violation{
			Rule:     v.Rule,
			Severity: parseSeverity(v.Severity),
			Message:  v.Message,
			Location: v.Location,
		})
	}

	// Parse diagnostics to get more info
	var diagnostics hollowcheckDiagnostics
	if result.Diagnostics != nil {
		json.Unmarshal(result.Diagnostics, &diagnostics)
	}

	grade := diagnostics.Grade
	if grade == "" {
		grade = calculateGrade(result.Score)
	}

	return &vtp.EvaluateResponse{
		Decision: vtp.GateDecision{
			Outcome:     outcome,
			Reason:      fmt.Sprintf("Score: %d, Grade: %s", result.Score, grade),
			Violations:  violations,
			RepairHints: result.RepairHints,
			Score:       result.Score,
			Grade:       grade,
		},
		EvaluationID: evalID,
		Evidence: &vtp.StructuralEvidence{
			Version:      diagnostics.Version,
			Score:        result.Score,
			Grade:        grade,
			Passed:       result.Passed,
			FilesScanned: diagnostics.FilesScanned,
			Findings:     convertFindings(result.Violations),
			Breakdown:    convertBreakdown(diagnostics.Breakdown),
		},
	}, nil
}

// Commit finalizes an evaluation and produces an attestation.
func (a *HollowcheckAdapter) Commit(ctx context.Context, evalID string, signerID string) (*vtp.Attestation, error) {
	a.mu.Lock()
	pe, ok := a.pendingEvals[evalID]
	if !ok {
		a.mu.Unlock()
		return nil, fmt.Errorf("evaluation %s not found", evalID)
	}
	delete(a.pendingEvals, evalID)
	a.mu.Unlock()

	// Parse diagnostics
	var diagnostics hollowcheckDiagnostics
	if pe.result.Diagnostics != nil {
		json.Unmarshal(pe.result.Diagnostics, &diagnostics)
	}

	grade := diagnostics.Grade
	if grade == "" {
		grade = calculateGrade(pe.result.Score)
	}

	// Build attestation
	att := vtp.NewAttestation(
		"", // WorkflowID set by orchestrator
		"hollowcheck",
		"Hollowcheck Gate",
		vtp.LayerStructural,
	)

	outcome := vtp.DecisionPass
	if !pe.result.Passed {
		outcome = vtp.DecisionFail
	}

	var violations []vtp.Violation
	for _, v := range pe.result.Violations {
		violations = append(violations, vtp.Violation{
			Rule:     v.Rule,
			Severity: parseSeverity(v.Severity),
			Message:  v.Message,
			Location: v.Location,
		})
	}

	att.Decision = vtp.GateDecision{
		Outcome:     outcome,
		Reason:      fmt.Sprintf("Score: %d, Grade: %s", pe.result.Score, grade),
		Violations:  violations,
		RepairHints: pe.result.RepairHints,
		Score:       pe.result.Score,
		Grade:       grade,
	}

	if err := att.SetEvidence(&vtp.StructuralEvidence{
		Version:      diagnostics.Version,
		Score:        pe.result.Score,
		Grade:        grade,
		Passed:       pe.result.Passed,
		FilesScanned: diagnostics.FilesScanned,
		Findings:     convertFindings(pe.result.Violations),
		Breakdown:    convertBreakdown(diagnostics.Breakdown),
	}); err != nil {
		return nil, fmt.Errorf("set evidence: %w", err)
	}

	att.ComputeHash()
	return att, nil
}

// Rollback aborts an evaluation.
func (a *HollowcheckAdapter) Rollback(ctx context.Context, evalID string, reason string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pendingEvals, evalID)
	return nil
}

// hollowcheckDiagnostics contains parsed hollowcheck output.
type hollowcheckDiagnostics struct {
	Version      string                   `json:"version"`
	Path         string                   `json:"path"`
	Contract     string                   `json:"contract"`
	Score        int                      `json:"score"`
	Grade        string                   `json:"grade"`
	Threshold    int                      `json:"threshold"`
	Passed       bool                     `json:"passed"`
	FilesScanned int                      `json:"files_scanned"`
	Breakdown    []hollowcheckBreakdown   `json:"breakdown"`
}

type hollowcheckBreakdown struct {
	Rule       string `json:"rule"`
	Points     int    `json:"points"`
	Violations int    `json:"violations"`
}

func parseSeverity(s string) vtp.Severity {
	switch s {
	case "error":
		return vtp.SeverityError
	case "warning":
		return vtp.SeverityWarning
	default:
		return vtp.SeverityInfo
	}
}

func calculateGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func convertFindings(violations []gate.Violation) []vtp.Finding {
	findings := make([]vtp.Finding, len(violations))
	for i, v := range violations {
		findings[i] = vtp.Finding{
			Rule:     v.Rule,
			Severity: parseSeverity(v.Severity),
			Message:  v.Message,
			File:     v.Location, // Location contains file:line
		}
	}
	return findings
}

func convertBreakdown(breakdown []hollowcheckBreakdown) *vtp.ScoreBreakdown {
	if len(breakdown) == 0 {
		return nil
	}

	categories := make(map[string]vtp.CategoryScore)
	for _, b := range breakdown {
		categories[b.Rule] = vtp.CategoryScore{
			Score:      b.Points,
			MaxScore:   100, // Assumed max
			Violations: b.Violations,
		}
	}

	return &vtp.ScoreBreakdown{
		Categories: categories,
	}
}
