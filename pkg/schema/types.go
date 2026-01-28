package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// === L1: Telemetry ===

type EntropyEventV1 struct {
	WindowMean   float32  `json:"window_mean"`
	Threshold    float32  `json:"threshold"`
	TokenIndices []uint32 `json:"token_indices"`
	Sustained    bool     `json:"sustained"`
}

type L1TelemetryArtifactV1 struct {
	Schema    string           `json:"schema"` // "zenedge.l1.telemetry.v1"
	StreamID  string           `json:"stream_id"`
	ModelID   string           `json:"model_id"`
	Events    []EntropyEventV1 `json:"events"`
	StartedAt int64            `json:"started_at"`
	EndedAt   int64            `json:"ended_at"`
}

// === L2: Attempt Log ===

type HollowgateResult struct {
	IntegrityCode uint16   `json:"integrity_code"`
	ViolationType string   `json:"violation_type"`
	RepairHints   []string `json:"repair_hints"`
	Complexity    float32  `json:"complexity"`
	Graftable     bool     `json:"graftable"`
}

type AttemptV1 struct {
	Schema     string           `json:"schema"`     // "flowgate.l2.attempt.v1"
	AttemptID  string           `json:"attempt_id"` // sha256(claim_hash + model_id + timestamp + out_hash)
	ClaimHash  string           `json:"claim_hash"`
	ModelID    string           `json:"model_id"`
	EntropyAvg float32          `json:"entropy_avg"`
	RiskScore  RiskScore        `json:"risk_score"`
	Hollow     HollowgateResult `json:"hollow_result"`
	OutputHash string           `json:"output_hash"`         // hash of produced output/code
	Grounding  json.RawMessage  `json:"grounding,omitempty"` // ExternalVerificationArtifact
	Timestamp  int64            `json:"timestamp"`
}

// === L3: Claim ===

type ClaimV1 struct {
	Schema     string `json:"schema"`     // "vtp.claim.v1"
	Hash       string `json:"claim_hash"` // sha256(canonical_json(ClaimV1_NoHash))
	Content    string `json:"content"`
	Domain     string `json:"domain"`
	InputsHash string `json:"inputs_hash"`
}

// === L4: Attestation ===

type EvidenceRef struct {
	Kind   string `json:"kind"` // "l1.telemetry", "l2.attempt", "output"
	SHA256 string `json:"sha256"`
}

type Provenance struct {
	Engine    string `json:"engine"`
	Host      string `json:"host"`
	Timestamp int64  `json:"timestamp"`
}

type Signature struct {
	Alg      string `json:"alg"` // "ed25519"
	PubKeyID string `json:"pubkey_id"`
	Sig      string `json:"sig"`
}

type AttestationV1 struct {
	Schema        string        `json:"schema"` // "vtp.attestation.v1"
	AttestationID string        `json:"attestation_id"`
	ClaimHash     string        `json:"claim_hash"`
	Decision      string        `json:"decision"` // "DecisionSuccess", "DecisionFail"
	PolicyID      string        `json:"policy_id"`
	EvidenceRefs  []EvidenceRef `json:"evidence_refs"`
	Provenance    Provenance    `json:"provenance"`
	Signature     *Signature    `json:"signature,omitempty"`
}

// === Canonical Hashing ===

// canonicalJSON returns a stable JSON representation (sorted keys).
// Go's encoding/json sorts map keys by default, ensuring stability for structs.
func canonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func ComputeSHA256(v any) (string, error) {
	data, err := canonicalJSON(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

func (c *ClaimV1) ComputeHash() error {
	// Create a temporary struct without the Hash field to avoid recursion/circularity
	// or just manually hash the fields that matter.
	// Best practice: Hash everything EXCEPT the hash field.
	// We can use a shadow struct.
	type ClaimContent struct {
		Schema     string `json:"schema"`
		Content    string `json:"content"`
		Domain     string `json:"domain"`
		InputsHash string `json:"inputs_hash"`
	}

	payload := ClaimContent{
		Schema:     c.Schema,
		Content:    c.Content,
		Domain:     c.Domain,
		InputsHash: c.InputsHash,
	}

	h, err := ComputeSHA256(payload)
	if err != nil {
		return err
	}
	c.Hash = h
	return nil
}

func (a *AttemptV1) ComputeID() error {
	payload := fmt.Sprintf("%s:%s:%d:%s", a.ClaimHash, a.ModelID, a.Timestamp, a.OutputHash)
	h := sha256.Sum256([]byte(payload))
	a.AttemptID = hex.EncodeToString(h[:])
	return nil
}

// === Risk Score ===

type RiskScore struct {
	SustainedEntropy    float32 `json:"sustained_entropy"`
	StructuralFlags     int     `json:"structural_flags"` // From Hollowgate
	Inconsistency       float32 `json:"inconsistency"`
	DomainRisk          float32 `json:"domain_risk"`
	GroundingConfidence float32 `json:"grounding_confidence"`
}

func (r RiskScore) Composite() float32 {
	// Weights: 0.3 (Entropy), 0.4 (Structural), 0.3 (Grounding)
	// Normalization assumptions:
	// Entropy: Max ~10.0. Normalized = val / 10.0
	// Structural: >0 is 1.0 (Risk)
	// Grounding: High Conf is Low Risk. Risk = 1.0 - Confidence

	normEntropy := r.SustainedEntropy / 10.0
	if normEntropy > 1.0 {
		normEntropy = 1.0
	}

	normStructural := float32(0.0)
	if r.StructuralFlags > 0 {
		normStructural = 1.0
	}

	normGroundingRisk := 1.0 - r.GroundingConfidence
	if normGroundingRisk < 0 {
		normGroundingRisk = 0
	}

	return (0.3 * normEntropy) + (0.4 * normStructural) + (0.3 * normGroundingRisk)
}

// === L3: Cache ===

type L3CacheEntry struct {
	ClaimHash      string `json:"claim_hash"`
	Domain         string `json:"domain"`
	Scope          string `json:"scope"`
	AttestationRef string `json:"attestation_ref"`
	ExpiresAt      int64  `json:"expires_at"`
}
