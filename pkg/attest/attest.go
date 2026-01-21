package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zen-systems/flowgate/pkg/evidence"
)

// AttestationV0 captures a minimal attestation for a stage run.
type AttestationV0 struct {
	Schema   string            `json:"schema"`
	Subject  Subject           `json:"subject"`
	Claim    Claim             `json:"claim"`
	Evidence Evidence          `json:"evidence"`
	Hashes   map[string]string `json:"hashes"`
}

// Subject identifies the attested stage.
type Subject struct {
	Workspace    string `json:"workspace"`
	PipelineFile string `json:"pipeline_file"`
	RunID        string `json:"run_id"`
	Stage        string `json:"stage"`
}

// Claim summarizes gate results.
type Claim struct {
	Passed    bool        `json:"passed"`
	GateCount int         `json:"gate_count"`
	Gated     bool        `json:"gated"`
	Gates     []GateClaim `json:"gates"`
}

// GateClaim summarizes a gate outcome.
type GateClaim struct {
	Name   string `json:"name"`
	Kind   string `json:"kind,omitempty"`
	Passed bool   `json:"passed"`
	Score  int    `json:"score"`
}

// Evidence references run artifacts.
type Evidence struct {
	RunJSON   string   `json:"run_json"`
	StageJSON string   `json:"stage_json"`
	Blobs     []string `json:"blobs"`
	GateLogs  []string `json:"gate_logs"`
}

// BuildAttestation builds a v0 attestation for a stage in a run directory.
func BuildAttestation(runDir, stageName string) (*AttestationV0, error) {
	if runDir == "" {
		return nil, fmt.Errorf("runDir is required")
	}
	if stageName == "" {
		return nil, fmt.Errorf("stageName is required")
	}

	runPath := filepath.Join(runDir, "run.json")
	stagePath := filepath.Join(runDir, "stages", fmt.Sprintf("%s.json", stageName))

	runData, err := os.ReadFile(runPath)
	if err != nil {
		return nil, err
	}
	stageData, err := os.ReadFile(stagePath)
	if err != nil {
		return nil, err
	}

	var runRecord evidence.RunRecord
	if err := json.Unmarshal(runData, &runRecord); err != nil {
		return nil, err
	}
	var stageRecord evidence.StageRecord
	if err := json.Unmarshal(stageData, &stageRecord); err != nil {
		return nil, err
	}

	gateClaims := make([]GateClaim, 0, len(stageRecord.GateResults))
	passed := true
	for _, g := range stageRecord.GateResults {
		gateClaims = append(gateClaims, GateClaim{
			Name:   g.Name,
			Kind:   g.Kind,
			Passed: g.Passed,
			Score:  g.Score,
		})
		if !g.Passed {
			passed = false
		}
	}
	sort.Slice(gateClaims, func(i, j int) bool {
		if gateClaims[i].Name == gateClaims[j].Name {
			return gateClaims[i].Kind < gateClaims[j].Kind
		}
		return gateClaims[i].Name < gateClaims[j].Name
	})

	blobs := collectStageBlobs(stageRecord)
	gateLogs, err := findGateLogs(runDir, stageName)
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]string)
	addHash := func(rel string) error {
		if rel == "" {
			return nil
		}
		if _, ok := hashes[rel]; ok {
			return nil
		}
		path, err := safeJoin(runDir, rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[rel] = hex.EncodeToString(sum[:])
		return nil
	}

	if err := addHash("run.json"); err != nil {
		return nil, err
	}
	if err := addHash(filepath.ToSlash(filepath.Join("stages", fmt.Sprintf("%s.json", stageName)))); err != nil {
		return nil, err
	}
	for _, blob := range blobs {
		if err := addHash(blob); err != nil {
			return nil, err
		}
	}
	for _, log := range gateLogs {
		if err := addHash(log); err != nil {
			return nil, err
		}
	}

	return &AttestationV0{
		Schema: "flowgate.attestation.v0",
		Subject: Subject{
			Workspace:    runRecord.Workspace,
			PipelineFile: runRecord.PipelineFile,
			RunID:        runRecord.ID,
			Stage:        stageName,
		},
		Claim: Claim{
			Passed:    passed,
			GateCount: len(gateClaims),
			Gated:     len(gateClaims) > 0,
			Gates:     gateClaims,
		},
		Evidence: Evidence{
			RunJSON:   "run.json",
			StageJSON: filepath.ToSlash(filepath.Join("stages", fmt.Sprintf("%s.json", stageName))),
			Blobs:     blobs,
			GateLogs:  gateLogs,
		},
		Hashes: hashes,
	}, nil
}

func collectStageBlobs(record evidence.StageRecord) []string {
	blobs := make([]string, 0, 4)
	if record.PromptRef != "" {
		blobs = append(blobs, record.PromptRef)
	}
	if record.OutputRef != "" {
		blobs = append(blobs, record.OutputRef)
	}
	for _, attempt := range record.Attempts {
		if attempt.PromptRef != "" {
			blobs = append(blobs, attempt.PromptRef)
		}
		if attempt.OutputRef != "" {
			blobs = append(blobs, attempt.OutputRef)
		}
	}

	seen := make(map[string]struct{}, len(blobs))
	unique := make([]string, 0, len(blobs))
	for _, blob := range blobs {
		if blob == "" {
			continue
		}
		blob = filepath.ToSlash(blob)
		if _, ok := seen[blob]; ok {
			continue
		}
		seen[blob] = struct{}{}
		unique = append(unique, blob)
	}
	sort.Strings(unique)
	return unique
}

func findGateLogs(runDir, stageName string) ([]string, error) {
	gatesDir := filepath.Join(runDir, "gates")
	entries, err := os.ReadDir(gatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	prefix := stageName + "-"
	logs := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			logs = append(logs, filepath.ToSlash(filepath.Join("gates", name)))
		}
	}
	sort.Strings(logs)
	return logs, nil
}

func safeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	normalized := filepath.FromSlash(rel)
	segments := strings.Split(normalized, string(filepath.Separator))
	for _, seg := range segments {
		if seg == ".." {
			return "", fmt.Errorf("path traversal detected")
		}
	}
	clean := filepath.Clean(normalized)
	if clean == "." {
		return "", fmt.Errorf("invalid path")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, clean)
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes run dir")
	}
	return targetAbs, nil
}
