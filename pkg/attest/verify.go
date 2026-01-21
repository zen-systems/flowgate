package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/zen-systems/flowgate/pkg/evidence"
)

// VerifyAttestation validates an attestation against the run directory.
func VerifyAttestation(att *AttestationV0, runDir string) error {
	if att == nil {
		return fmt.Errorf("attestation is required")
	}
	if runDir == "" {
		return fmt.Errorf("runDir is required")
	}
	mode, err := parseSchemaMode(att.Schema)
	if err != nil {
		return err
	}

	for rel, expected := range att.Hashes {
		path, err := safeJoin(runDir, rel)
		if err != nil {
			return fmt.Errorf("invalid hash path %q: %w", rel, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("missing evidence file %s: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		actual := hex.EncodeToString(sum[:])
		if actual != expected {
			return fmt.Errorf("hash mismatch for %s", rel)
		}
	}

	stagePath, err := safeJoin(runDir, att.Evidence.StageJSON)
	if err != nil {
		return fmt.Errorf("invalid stage json path: %w", err)
	}
	stageData, err := os.ReadFile(stagePath)
	if err != nil {
		return fmt.Errorf("read stage json: %w", err)
	}

	var stageRecord evidence.StageRecord
	if err := json.Unmarshal(stageData, &stageRecord); err != nil {
		return fmt.Errorf("parse stage json: %w", err)
	}

	if err := verifyClaim(att, stageRecord, mode); err != nil {
		return err
	}

	return nil
}

func VerifyAttestationFile(attestationPath, runDir string) error {
	data, err := os.ReadFile(attestationPath)
	if err != nil {
		return err
	}
	var att AttestationV0
	if err := json.Unmarshal(data, &att); err != nil {
		return err
	}
	return VerifyAttestation(&att, runDir)
}

func verifyClaim(att *AttestationV0, stageRecord evidence.StageRecord, mode schemaMode) error {
	expectedGateCount := len(stageRecord.GateResults)
	claimPassed := true
	for _, g := range stageRecord.GateResults {
		if !g.Passed {
			claimPassed = false
			break
		}
	}

	if mode == schemaModeV0 {
		expectedGated := expectedGateCount > 0
		if att.Claim.GateCount != expectedGateCount || att.Claim.Gated != expectedGated {
			return fmt.Errorf("claim gating mismatch")
		}
		if !expectedGated {
			claimPassed = true
		}
		if att.Claim.Passed != claimPassed {
			return fmt.Errorf("claim.passed mismatch")
		}
	} else {
		lastAttemptSucceeded := false
		if len(stageRecord.Attempts) > 0 {
			last := stageRecord.Attempts[len(stageRecord.Attempts)-1]
			lastAttemptSucceeded = last.Succeeded && last.ApplyError == ""
		} else {
			lastAttemptSucceeded = len(stageRecord.GateResults) > 0
		}
		if !lastAttemptSucceeded {
			claimPassed = false
		}
		if att.Claim.Passed != claimPassed {
			return fmt.Errorf("claim.passed mismatch")
		}
	}

	if len(att.Claim.Gates) != len(stageRecord.GateResults) {
		return fmt.Errorf("claim.gates mismatch")
	}

	attGates := append([]GateClaim{}, att.Claim.Gates...)
	stageGates := make([]GateClaim, 0, len(stageRecord.GateResults))
	for _, g := range stageRecord.GateResults {
		stageGates = append(stageGates, GateClaim{
			Name:   g.Name,
			Kind:   g.Kind,
			Passed: g.Passed,
			Score:  g.Score,
		})
	}

	sort.Slice(attGates, func(i, j int) bool {
		if attGates[i].Name == attGates[j].Name {
			return attGates[i].Kind < attGates[j].Kind
		}
		return attGates[i].Name < attGates[j].Name
	})
	sort.Slice(stageGates, func(i, j int) bool {
		if stageGates[i].Name == stageGates[j].Name {
			return stageGates[i].Kind < stageGates[j].Kind
		}
		return stageGates[i].Name < stageGates[j].Name
	})

	for i := range attGates {
		a := attGates[i]
		s := stageGates[i]
		if a.Name != s.Name || a.Kind != s.Kind || a.Passed != s.Passed || a.Score != s.Score {
			return fmt.Errorf("claim gate mismatch for %s", a.Name)
		}
	}

	return nil
}

type schemaMode int

const (
	schemaModeLegacy schemaMode = iota
	schemaModeV0
)

func parseSchemaMode(schema string) (schemaMode, error) {
	if schema == "" {
		return schemaModeLegacy, nil
	}
	if schema == "flowgate.attestation.v0" {
		return schemaModeV0, nil
	}
	return schemaModeLegacy, fmt.Errorf("unknown attestation schema: %s", schema)
}
