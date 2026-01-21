package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/evidence"
	"github.com/zen-systems/flowgate/pkg/gate"
	"github.com/zen-systems/flowgate/pkg/repair"
	"github.com/zen-systems/flowgate/pkg/workspace"
)

// RunOptions configures pipeline execution.
type RunOptions struct {
	Input         string
	WorkspacePath string
	EvidenceDir   string
	PipelinePath  string
	Logger        func(format string, args ...any)
}

// RunResult captures pipeline outputs.
type RunResult struct {
	RunID       string
	EvidenceDir string
	Stages      map[string]*StageResult
}

// StageResult captures execution results for a stage.
type StageResult struct {
	Name        string
	Artifact    *artifact.Artifact
	GateResults []GateResult
	ApplyResult *workspace.ApplyResult
	Duration    time.Duration
}

// GateResult captures a gate evaluation with metadata.
type GateResult struct {
	Name     string
	Result   *gate.GateResult
	Error    error
	Duration time.Duration
}

// Run executes the pipeline with the given adapters and options.
func Run(ctx context.Context, pipeline *Pipeline, opts RunOptions) (*RunResult, error) {
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline is required")
	}
	if err := pipeline.Validate(); err != nil {
		return nil, err
	}

	adapters := pipeline.Adapters
	if len(adapters) == 0 {
		return nil, fmt.Errorf("no adapters configured")
	}

	workspacePath := opts.WorkspacePath
	if workspacePath == "" {
		workspacePath = pipeline.Workspace.Path
	}
	if workspacePath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workspacePath = cwd
	}

	writer, err := prepareEvidenceWriter(opts.EvidenceDir, workspacePath)
	if err != nil {
		return nil, err
	}

	runID := filepath.Base(writer.RunDir())
	runRecord := evidence.RunRecord{
		ID:           runID,
		Timestamp:    time.Now().UTC(),
		PipelineFile: opts.PipelinePath,
		InputHash:    hashString(opts.Input),
		Workspace:    workspacePath,
		ToolVersions: map[string]string{"go": runtime.Version()},
	}
	if err := writer.WriteRun(runRecord); err != nil {
		return nil, err
	}

	results := make(map[string]*StageResult)
	artifacts := make(map[string]ArtifactTemplateData)
	stagesLegacy := make(map[string]map[string]string)

	for _, stage := range pipeline.Stages {
		stageResult, stageRecord, err := runStage(ctx, stage, adapters, pipeline, opts.Input, workspacePath, artifacts, stagesLegacy)
		if stageRecord != nil {
			stageRecord.Name = stage.Name
			if writeErr := writer.WriteStage(*stageRecord); writeErr != nil {
				return nil, writeErr
			}
		}
		if err != nil {
			return nil, err
		}

		if err := writeGateLogs(writer, stage.Name, stageResult.GateResults); err != nil {
			return nil, err
		}

		results[stage.Name] = stageResult
		artifacts[stage.Name] = ArtifactTemplateData{Text: stageResult.Artifact.Content, Output: stageResult.Artifact.Content, Hash: stageResult.Artifact.Hash}
		stagesLegacy[stage.Name] = map[string]string{"output": stageResult.Artifact.Content}
	}

	return &RunResult{
		RunID:       runID,
		EvidenceDir: writer.RunDir(),
		Stages:      results,
	}, nil
}

func runStage(
	ctx context.Context,
	stage *Stage,
	adapters map[string]adapter.Adapter,
	pipeline *Pipeline,
	input string,
	workspacePath string,
	artifacts map[string]ArtifactTemplateData,
	stagesLegacy map[string]map[string]string,
) (*StageResult, *evidence.StageRecord, error) {
	if stage == nil {
		return nil, nil, fmt.Errorf("stage is nil")
	}

	start := time.Now()
	stageRecord := &evidence.StageRecord{}

	adapterName := stage.Adapter
	if adapterName == "" {
		adapterName = pipeline.DefaultAdapter
	}
	if adapterName == "" {
		adapterName = pickSingleAdapter(adapters)
	}
	adapterImpl, ok := adapters[adapterName]
	if !ok {
		return nil, stageRecord, fmt.Errorf("adapter %s not found", adapterName)
	}

	model := stage.Model
	if model == "" {
		model = pipeline.DefaultModel
	}
	if model == "" {
		models := adapterImpl.Models()
		if len(models) > 0 {
			model = models[0]
		}
	}
	if model == "" {
		return nil, stageRecord, fmt.Errorf("model not specified for stage %s", stage.Name)
	}

	prompt, err := renderPrompt(stage.Prompt, input, artifacts, stagesLegacy)
	if err != nil {
		return nil, stageRecord, fmt.Errorf("render prompt for stage %s: %w", stage.Name, err)
	}

	stageRecord.Adapter = adapterName
	stageRecord.Model = model
	stageRecord.Prompt = truncateForEvidence(prompt, 4096)
	if stageRecord.Prompt != prompt {
		stageRecord.PromptHash = hashString(prompt)
	}

	attempts := stage.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastArtifact *artifact.Artifact
	var lastGateResults []GateResult
	var lastApplyResult *workspace.ApplyResult
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStart := time.Now()
		art, err := adapterImpl.Generate(ctx, model, prompt)
		if err != nil {
			lastErr = fmt.Errorf("stage %s adapter error: %w", stage.Name, err)
			return nil, stageRecord, lastErr
		}
		lastArtifact = art

		applyResult, applyErr := applyIfNeeded(stage, workspacePath, art)
		lastApplyResult = applyResult

		gateResults, gateErr := evaluateGates(stage, pipeline, art, workspacePath)
		lastGateResults = gateResults

		succeeded := applyErr == nil && gateErr == nil
		attemptRecord := evidence.AttemptRecord{
			Attempt:        attempt,
			PromptHash:     hashString(prompt),
			GateResults:    evidenceGateRecords(gateResults),
			Succeeded:      succeeded,
			DurationMillis: time.Since(attemptStart).Milliseconds(),
		}
		if applyErr != nil {
			attemptRecord.ApplyError = applyErr.Error()
		}
		stageRecord.Attempts = append(stageRecord.Attempts, attemptRecord)

		if succeeded {
			lastErr = nil
			break
		}

		if attempt == attempts {
			if applyErr != nil {
				lastErr = applyErr
			} else if gateErr != nil {
				lastErr = gateErr
			} else {
				lastErr = fmt.Errorf("stage %s failed", stage.Name)
			}
			break
		}

		failureResult := consolidateGateFailures(gateResults, applyErr)
		prompt = repair.GenerateRepairPrompt(art, failureResult)
	}

	if lastErr != nil {
		return nil, stageRecord, lastErr
	}

	stageRecord.Output = truncateForEvidence(lastArtifact.Content, 4096)
	if stageRecord.Output != lastArtifact.Content {
		stageRecord.OutputHash = hashString(lastArtifact.Content)
	}
	stageRecord.GateResults = evidenceGateRecords(lastGateResults)
	stageRecord.Artifacts = map[string]string{
		"text": lastArtifact.Content,
		"hash": lastArtifact.Hash,
	}
	if lastApplyResult != nil {
		stageRecord.ApplyResult = &evidence.ApplyRecord{
			AppliedFiles:    lastApplyResult.AppliedFiles,
			DeletedFiles:    lastApplyResult.DeletedFiles,
			UsedUnifiedDiff: lastApplyResult.UsedUnifiedDiff,
		}
	}
	stageRecord.DurationMillis = time.Since(start).Milliseconds()

	return &StageResult{
		Name:        stage.Name,
		Artifact:    lastArtifact,
		GateResults: lastGateResults,
		ApplyResult: lastApplyResult,
		Duration:    time.Since(start),
	}, stageRecord, nil
}

func applyIfNeeded(stage *Stage, workspacePath string, art *artifact.Artifact) (*workspace.ApplyResult, error) {
	if !stage.Apply {
		return nil, nil
	}
	result, err := workspace.ApplyOutput(workspacePath, art.Content)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func evaluateGates(stage *Stage, pipeline *Pipeline, art *artifact.Artifact, workspacePath string) ([]GateResult, error) {
	if len(stage.Gates) == 0 {
		return nil, nil
	}

	gateInstances, err := buildGateInstances(pipeline, stage.Gates, workspacePath)
	if err != nil {
		return nil, err
	}

	var results []GateResult
	for _, gateInstance := range gateInstances {
		start := time.Now()
		res, err := gateInstance.Evaluate(art)
		results = append(results, GateResult{
			Name:     gateInstance.Name(),
			Result:   res,
			Error:    err,
			Duration: time.Since(start),
		})
		if err != nil {
			return results, fmt.Errorf("gate %s error: %w", gateInstance.Name(), err)
		}
		if res != nil && !res.Passed {
			return results, fmt.Errorf("gate %s failed", gateInstance.Name())
		}
	}

	return results, nil
}

func buildGateInstances(pipeline *Pipeline, gateNames []string, workspacePath string) ([]gate.Gate, error) {
	instances := make([]gate.Gate, 0, len(gateNames))
	for _, name := range gateNames {
		if name == "hollowcheck" {
			instances = append(instances, gate.NewHollowCheckGate("", ""))
			continue
		}

		def, ok := pipeline.Gates[name]
		if !ok {
			return nil, fmt.Errorf("gate %s not defined", name)
		}
		switch strings.ToLower(def.Type) {
		case "hollowcheck":
			instances = append(instances, gate.NewHollowCheckGate(def.BinaryPath, def.ContractPath))
		case "command":
			workdir := def.Workdir
			if workdir == "" {
				workdir = workspacePath
			} else if !filepath.IsAbs(workdir) {
				workdir = filepath.Join(workspacePath, workdir)
			}
			g, err := gate.NewCommandGate(name, def.Command, workdir)
			if err != nil {
				return nil, err
			}
			instances = append(instances, g)
		default:
			return nil, fmt.Errorf("unsupported gate type %s", def.Type)
		}
	}

	return instances, nil
}

func consolidateGateFailures(results []GateResult, applyErr error) *gate.GateResult {
	if applyErr != nil {
		return gate.NewFailingResult(100, []gate.Violation{
			{
				Rule:     "apply_failed",
				Severity: "error",
				Message:  applyErr.Error(),
			},
		}, nil)
	}

	var violations []gate.Violation
	var hints []string
	for _, result := range results {
		if result.Result == nil || result.Result.Passed {
			continue
		}
		violations = append(violations, result.Result.Violations...)
		hints = append(hints, result.Result.RepairHints...)
	}

	if len(violations) == 0 {
		violations = append(violations, gate.Violation{
			Rule:     "gate_failed",
			Severity: "error",
			Message:  "gate failed without specific violations",
		})
	}

	return gate.NewFailingResult(100, violations, hints)
}

func renderPrompt(prompt string, input string, artifacts map[string]ArtifactTemplateData, stages map[string]map[string]string) (string, error) {
	data := map[string]any{
		"Input":     input,
		"input":     input,
		"Artifacts": artifacts,
		"artifacts": artifacts,
		"Stages":    stages,
		"stages":    stages,
	}

	tmpl, err := template.New("prompt").Parse(prompt)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func pickSingleAdapter(adapters map[string]adapter.Adapter) string {
	if len(adapters) != 1 {
		return ""
	}
	for key := range adapters {
		return key
	}
	return ""
}

func prepareEvidenceWriter(baseDir, workspacePath string) (*evidence.Writer, error) {
	if baseDir == "" {
		baseDir = filepath.Join(workspacePath, ".flowgate", "runs")
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	runID := fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102T150405Z"), randomSuffix())
	return evidence.NewWriter(baseDir, runID)
}

func writeGateLogs(writer *evidence.Writer, stageName string, results []GateResult) error {
	for _, result := range results {
		if result.Result == nil || result.Result.Diagnostics == nil {
			continue
		}
		logContent := fmt.Sprintf("command: %s\nexit: %d\n\nstdout:\n%s\n\nstderr:\n%s\n",
			strings.Join(result.Result.Diagnostics.Command, " "),
			result.Result.Diagnostics.ExitCode,
			result.Result.Diagnostics.Stdout,
			result.Result.Diagnostics.Stderr,
		)
		if err := writer.WriteGateLog(stageName, result.Name, logContent); err != nil {
			return err
		}
	}
	return nil
}

func evidenceGateRecords(results []GateResult) []evidence.GateRecord {
	records := make([]evidence.GateRecord, 0, len(results))
	for _, result := range results {
		record := evidence.GateRecord{
			Name:           result.Name,
			DurationMillis: result.Duration.Milliseconds(),
		}
		if result.Error != nil {
			record.Error = result.Error.Error()
		}
		if result.Result != nil {
			record.Passed = result.Result.Passed
			record.Score = result.Result.Score
			for _, v := range result.Result.Violations {
				record.Violations = append(record.Violations, evidence.Violation{
					Rule:       v.Rule,
					Severity:   v.Severity,
					Message:    v.Message,
					Location:   v.Location,
					Suggestion: v.Suggestion,
				})
			}
			record.RepairHints = append(record.RepairHints, result.Result.RepairHints...)
			if diag := result.Result.Diagnostics; diag != nil {
				record.Diagnostics = &evidence.Diagnostics{
					Command:  diag.Command,
					Workdir:  diag.Workdir,
					Stdout:   diag.Stdout,
					Stderr:   diag.Stderr,
					ExitCode: diag.ExitCode,
					Duration: diag.Duration.String(),
				}
			}
		}
		records = append(records, record)
	}
	return records
}

// ArtifactTemplateData exposes stage output to templates.
type ArtifactTemplateData struct {
	Text   string
	Output string
	Hash   string
}

func hashString(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

func truncateForEvidence(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func randomSuffix() string {
	now := time.Now().UTC().UnixNano()
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d", now)))
	return hex.EncodeToString(sum[:4])
}
