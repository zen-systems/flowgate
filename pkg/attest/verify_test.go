package attest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/evidence"
)

func TestVerifyAttestationSuccess(t *testing.T) {
	runDir := t.TempDir()
	setupRunDir(t, runDir)

	att, err := BuildAttestation(runDir, "build")
	if err != nil {
		t.Fatalf("build attestation: %v", err)
	}

	if err := VerifyAttestation(att, runDir); err != nil {
		t.Fatalf("verify attestation: %v", err)
	}
}

func TestVerifyAttestationLegacy(t *testing.T) {
	runDir := t.TempDir()
	setupRunDir(t, runDir)

	att, err := BuildAttestation(runDir, "build")
	if err != nil {
		t.Fatalf("build attestation: %v", err)
	}
	att.Schema = ""
	att.Claim.Gated = false
	att.Claim.GateCount = 0

	if err := VerifyAttestation(att, runDir); err != nil {
		t.Fatalf("verify legacy attestation: %v", err)
	}
}

func TestVerifyAttestationHashMismatch(t *testing.T) {
	runDir := t.TempDir()
	setupRunDir(t, runDir)

	att, err := BuildAttestation(runDir, "build")
	if err != nil {
		t.Fatalf("build attestation: %v", err)
	}

	blobPath := filepath.Join(runDir, "blobs", "prompt-aaa.txt")
	if err := os.WriteFile(blobPath, []byte("tampered"), 0644); err != nil {
		t.Fatalf("tamper blob: %v", err)
	}

	if err := VerifyAttestation(att, runDir); err == nil {
		t.Fatalf("expected hash mismatch")
	}
}

func TestVerifyAttestationUnknownSchema(t *testing.T) {
	runDir := t.TempDir()
	setupRunDir(t, runDir)

	att, err := BuildAttestation(runDir, "build")
	if err != nil {
		t.Fatalf("build attestation: %v", err)
	}
	att.Schema = "flowgate.attestation.v99"

	if err := VerifyAttestation(att, runDir); err == nil {
		t.Fatalf("expected unknown schema error")
	}
}

func TestVerifyAttestationRejectsTraversal(t *testing.T) {
	runDir := t.TempDir()
	setupRunDir(t, runDir)

	att := &AttestationV0{
		Evidence: Evidence{
			RunJSON:   "run.json",
			StageJSON: "stages/build.json",
		},
		Hashes: map[string]string{
			"../x": "deadbeef",
		},
	}

	if err := VerifyAttestation(att, runDir); err == nil {
		t.Fatalf("expected traversal error")
	}
}

func setupRunDir(t *testing.T, runDir string) {
	t.Helper()
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

	if err := os.WriteFile(filepath.Join(runDir, "blobs", "prompt-aaa.txt"), []byte("prompt"), 0644); err != nil {
		t.Fatalf("write prompt blob: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "blobs", "output-bbb.txt"), []byte("output"), 0644); err != nil {
		t.Fatalf("write output blob: %v", err)
	}

	stageRecord := evidence.StageRecord{
		Name:      "build",
		PromptRef: "blobs/prompt-aaa.txt",
		OutputRef: "blobs/output-bbb.txt",
		GateResults: []evidence.GateRecord{
			{
				Name:   "gate",
				Kind:   "command",
				Passed: true,
				Score:  0,
			},
		},
		Attempts: []evidence.AttemptRecord{
			{
				Succeeded: true,
			},
		},
	}
	writeJSONFile(t, filepath.Join(runDir, "stages", "build.json"), stageRecord)

	if err := os.WriteFile(filepath.Join(runDir, "gates", "build-gate.log"), []byte("log"), 0644); err != nil {
		t.Fatalf("write gate log: %v", err)
	}
}
