package vtp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zen-systems/vtp"
)

// WorkflowConfig configures the VTP workflow.
type WorkflowConfig struct {
	// TavilyAPIKey for provenance gate.
	TavilyAPIKey string

	// HollowcheckPath is the path to the hollowcheck binary.
	HollowcheckPath string

	// ContractPath is the path to the hollowcheck contract.
	ContractPath string

	// SignerID identifies who is signing attestations.
	SignerID string

	// MaxAttempts is the maximum repair attempts for hollowcheck.
	MaxAttempts int

	// LLMAdapter for generating plans and patches (interface to be defined).
	LLMAdapter LLMAdapter
}

// LLMAdapter interface for LLM interactions.
type LLMAdapter interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// WorkflowResult contains the output of a workflow run.
type WorkflowResult struct {
	Status           WorkflowStatus
	Questions        []string                  // For NeedsClarification
	UnverifiedClaims []string                  // For ProvenanceFailed
	Conflicts        []string                  // For NeedsReview
	Violations       []vtp.Violation           // For QualityFailed
	Patch            string                    // For Success
	Attestation      *vtp.Workflow             // The attestation chain
	Ledger           *vtp.ExternalityLedger    // Tracked externalities
}

// WorkflowStatus represents the outcome of a workflow.
type WorkflowStatus string

const (
	StatusSuccess            WorkflowStatus = "success"
	StatusNeedsClarification WorkflowStatus = "needs_clarification"
	StatusProvenanceFailed   WorkflowStatus = "provenance_failed"
	StatusNeedsReview        WorkflowStatus = "needs_review"
	StatusQualityFailed      WorkflowStatus = "quality_failed"
	StatusError              WorkflowStatus = "error"
)

// RunSpecProvPatchWorkflow executes the Spec→Prov→Patch workflow.
func RunSpecProvPatchWorkflow(
	ctx context.Context,
	request string,
	workspace string,
	cfg WorkflowConfig,
) (*WorkflowResult, error) {
	// Initialize VTP components
	registry := vtp.NewRegistry()

	// Note: spec-gate and provenance-gate would be registered here
	// For now, we only have hollowcheck
	registry.Register(NewHollowcheckAdapter(cfg.HollowcheckPath, cfg.ContractPath))

	orch := vtp.NewOrchestrator(vtp.OrchestratorConfig{
		Registry: registry,
		SignerID: cfg.SignerID,
	})

	ledger := vtp.NewExternalityLedger()
	workflowID := vtp.GenerateWorkflowID()

	if _, err := orch.StartWorkflow(workflowID); err != nil {
		return nil, fmt.Errorf("start workflow: %w", err)
	}

	maxAttempts := cfg.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	// For now, since spec-gate and provenance-gate are separate repos,
	// we'll skip those steps and just run hollowcheck.
	// In production, you'd import and register those gates.

	// === STEP: HollowGate (with repair loop) ===
	var hollowResp *vtp.EvaluateResponse
	patch := request // In production, this would be generated from LLM

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var err error
		hollowResp, err = orch.EvaluateGate(ctx, workflowID, "hollowcheck", vtp.Artifact{
			Content:     patch,
			ContentType: "text/x-patch",
		}, nil)
		if err != nil {
			return &WorkflowResult{
				Status: StatusError,
				Ledger: ledger,
			}, err
		}

		if hollowResp.Decision.Outcome == vtp.DecisionPass {
			break
		}

		if attempt < maxAttempts {
			// Repair: feed violations back to LLM
			if cfg.LLMAdapter != nil {
				repairPrompt := generateRepairPrompt(patch, hollowResp.Decision.Violations, hollowResp.Decision.RepairHints)
				newPatch, err := cfg.LLMAdapter.Generate(ctx, repairPrompt)
				if err == nil {
					patch = newPatch
				}
			}
			orch.RollbackEvaluation(ctx, hollowResp.EvaluationID, "repair attempt")
		}
	}

	if hollowResp.Decision.Outcome != vtp.DecisionPass {
		return &WorkflowResult{
			Status:     StatusQualityFailed,
			Violations: hollowResp.Decision.Violations,
			Ledger:     ledger,
		}, nil
	}
	orch.CommitEvaluation(ctx, hollowResp.EvaluationID)

	// === Finalize ===
	workflow, err := orch.FinalizeWorkflow(workflowID)
	if err != nil {
		return &WorkflowResult{
			Status: StatusError,
			Ledger: ledger,
		}, err
	}

	// === Record Externalities ===
	exts := detectAcceptedExternalities(hollowResp)
	for _, ext := range exts {
		ledger.Record(&ext)
	}

	return &WorkflowResult{
		Status:      StatusSuccess,
		Patch:       patch,
		Attestation: workflow,
		Ledger:      ledger,
	}, nil
}

func generateRepairPrompt(patch string, violations []vtp.Violation, hints []string) string {
	prompt := "The following code has quality violations that need to be fixed:\n\n"
	prompt += "```\n" + patch + "\n```\n\n"
	prompt += "Violations:\n"
	for _, v := range violations {
		prompt += fmt.Sprintf("- [%s] %s at %s: %s\n", v.Severity, v.Rule, v.Location, v.Message)
	}
	if len(hints) > 0 {
		prompt += "\nRepair hints:\n"
		for _, h := range hints {
			prompt += fmt.Sprintf("- %s\n", h)
		}
	}
	prompt += "\nPlease fix these issues and return the corrected code."
	return prompt
}

func detectAcceptedExternalities(resp *vtp.EvaluateResponse) []vtp.Externality {
	var externalities []vtp.Externality

	// Look for warnings that were accepted (passed despite warnings)
	if resp.Decision.Outcome == vtp.DecisionPass {
		for _, v := range resp.Decision.Violations {
			if v.Severity == vtp.SeverityWarning {
				externalities = append(externalities, vtp.Externality{
					Type:        vtp.ExternalityTODO,
					Description: v.Message,
					Location:    v.Location,
					GateID:      "hollowcheck",
					Severity:    v.Severity,
				})
			}
		}
	}

	return externalities
}

// ExtractContract extracts a permission contract from epistemic evidence.
func ExtractContract(evidence json.RawMessage) (*vtp.PermissionContract, error) {
	var epEvidence vtp.EpistemicEvidence
	if err := json.Unmarshal(evidence, &epEvidence); err != nil {
		return nil, err
	}
	return epEvidence.Contract, nil
}
