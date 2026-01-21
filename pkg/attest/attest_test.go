package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/evidence"
)

func TestBuildAttestation(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "stages"), 0755); err != nil {
		t.Fatalf("mkdir stages: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "blobs"), 0755); err != nil {
		t.Fatalf("mkdir blobs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "gates"), 0755); err != nil {
		t.Fatalf("mkdir gates: %v", err)
	}

	runRecord := evidence.RunRecord{
		ID:           "run-1",
		PipelineFile: "pipelines/examples/feature.yaml",
		Workspace:    "/repo",
	}
	writeJSONFile(t, filepath.Join(runDir, "run.json"), runRecord)

	promptPath := filepath.Join("blobs", "prompt-aaa.txt")
	outputPath := filepath.Join("blobs", "output-bbb.txt")
	if err := os.WriteFile(filepath.Join(runDir, promptPath), []byte("prompt"), 0644); err != nil {
		t.Fatalf("write prompt blob: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, outputPath), []byte("output"), 0644); err != nil {
		t.Fatalf("write output blob: %v", err)
	}

	stageRecord := evidence.StageRecord{
		Name:      "build",
		PromptRef: promptPath,
		OutputRef: outputPath,
		GateResults: []evidence.GateRecord{
			{
				Name:   "beta",
				Kind:   "command",
				Passed: true,
				Score:  0,
			},
			{
				Name:   "alpha",
				Kind:   "hollowcheck",
				Passed: true,
				Score:  1,
			},
		},
	}
	writeJSONFile(t, filepath.Join(runDir, "stages", "build.json"), stageRecord)

	gateLogPath := filepath.Join(runDir, "gates", "build-gate.log")
	if err := os.WriteFile(gateLogPath, []byte("log"), 0644); err != nil {
		t.Fatalf("write gate log: %v", err)
	}

	att, err := BuildAttestation(runDir, "build")
	if err != nil {
		t.Fatalf("build attestation: %v", err)
	}

	if att.Schema != "flowgate.attestation.v0" {
		t.Fatalf("unexpected schema")
	}
	if att.Subject.RunID != "run-1" || att.Subject.Stage != "build" {
		t.Fatalf("unexpected subject")
	}
	if !att.Claim.Passed || !att.Claim.Gated || att.Claim.GateCount != 2 {
		t.Fatalf("unexpected claim semantics")
	}
	if len(att.Claim.Gates) != 2 || att.Claim.Gates[0].Name != "alpha" || att.Claim.Gates[1].Name != "beta" {
		t.Fatalf("expected sorted gate claims")
	}
	if len(att.Evidence.Blobs) != 2 {
		t.Fatalf("expected blob refs")
	}
	if len(att.Evidence.GateLogs) != 1 {
		t.Fatalf("expected gate logs")
	}

	assertHashFile(t, att.Hashes, runDir, "run.json")
	assertHashFile(t, att.Hashes, runDir, filepath.ToSlash(filepath.Join("stages", "build.json")))
	assertHashBytes(t, att.Hashes, promptPath, []byte("prompt"))
	assertHashBytes(t, att.Hashes, outputPath, []byte("output"))
	assertHashFile(t, att.Hashes, runDir, filepath.ToSlash(filepath.Join("gates", "build-gate.log")))
}

func TestBuildAttestationRejectsTraversal(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "stages"), 0755); err != nil {
		t.Fatalf("mkdir stages: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "blobs"), 0755); err != nil {
		t.Fatalf("mkdir blobs: %v", err)
	}

	runRecord := evidence.RunRecord{ID: "run-1"}
	writeJSONFile(t, filepath.Join(runDir, "run.json"), runRecord)

	stageRecord := evidence.StageRecord{
		Name:      "build",
		PromptRef: "../secrets.txt",
	}
	writeJSONFile(t, filepath.Join(runDir, "stages", "build.json"), stageRecord)

	if _, err := BuildAttestation(runDir, "build"); err == nil {
		t.Fatalf("expected traversal error")
	}
}

func assertHashFile(t *testing.T, hashes map[string]string, runDir, rel string) {
	data, err := os.ReadFile(filepath.Join(runDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	assertHashBytes(t, hashes, rel, data)
}

func assertHashBytes(t *testing.T, hashes map[string]string, rel string, content []byte) {
	sum := sha256.Sum256(content)
	expected := hex.EncodeToString(sum[:])
	actual, ok := hashes[rel]
	if !ok {
		t.Fatalf("missing hash for %s", rel)
	}
	if actual != expected {
		t.Fatalf("hash mismatch for %s", rel)
	}
}
