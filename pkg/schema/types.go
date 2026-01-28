package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	SchemaL1TelemetryArtifactV1 = "zenedge.l1.telemetry.v1"
	SchemaAttemptV1            = "flowgate.l2.attempt.v1"
	SchemaClaimV1              = "vtp.claim.v1"
	SchemaAttestationV1        = "vtp.attestation.v1"
	InputsHashUnknown          = "inputs.unknown"
	SignatureAlgEd25519        = "ed25519"
)

type AttestationDecision string

const (
	DecisionSuccess  AttestationDecision = "DecisionSuccess"
	DecisionDenied   AttestationDecision = "DecisionDenied"
	DecisionDegraded AttestationDecision = "DecisionDegraded"
	DecisionFail     AttestationDecision = "DecisionFail"
)

type EvidenceKind string

const (
	EvidenceKindL1Telemetry        EvidenceKind = "l1.telemetry"
	EvidenceKindL2Attempt          EvidenceKind = "l2.attempt"
	EvidenceKindOutput             EvidenceKind = "output"
	EvidenceKindEpistemicGrounding EvidenceKind = "epistemic.grounding"
)

// === L1: Telemetry ===

type EntropyEventV1 struct {
	WindowMean   float32  `json:"window_mean"`
	Threshold    float32  `json:"threshold"`
	TokenIndices []uint32 `json:"token_indices"`
	Sustained    bool     `json:"sustained"`
}

type L1TelemetryArtifactV1 struct {
	Schema    string           `json:"schema"` // SchemaL1TelemetryArtifactV1
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
	Schema     string           `json:"schema"`     // SchemaAttemptV1
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
	Schema     string `json:"schema"`     // SchemaClaimV1
	Hash       string `json:"claim_hash"` // sha256(canonical_json(ClaimV1_NoHash))
	Content    string `json:"content"`
	Domain     string `json:"domain"`
	InputsHash string `json:"inputs_hash"`
}

// === L4: Attestation ===

type EvidenceRef struct {
	Kind   string `json:"kind"` // EvidenceKind*
	SHA256 string `json:"sha256"`
}

type Provenance struct {
	Engine    string `json:"engine"`
	Host      string `json:"host"`
	Timestamp int64  `json:"timestamp"`
}

type Signature struct {
	Alg      string `json:"alg"` // SignatureAlgEd25519
	PubKeyID string `json:"pubkey_id"`
	Sig      string `json:"sig"`
}

type AttestationV1 struct {
	Schema        string        `json:"schema"` // SchemaAttestationV1
	AttestationID string        `json:"attestation_id"`
	ClaimHash     string        `json:"claim_hash"`
	Decision      string        `json:"decision"` // AttestationDecision
	PolicyID      string        `json:"policy_id"`
	EvidenceRefs  []EvidenceRef `json:"evidence_refs"`
	Provenance    Provenance    `json:"provenance"`
	Signature     *Signature    `json:"signature,omitempty"`
}

func (c *ClaimV1) Validate() error {
	if c.Schema != SchemaClaimV1 {
		return fmt.Errorf("claim schema must be %q", SchemaClaimV1)
	}
	if strings.TrimSpace(c.Content) == "" {
		return fmt.Errorf("claim content required")
	}
	if strings.TrimSpace(c.Domain) == "" {
		return fmt.Errorf("claim domain required")
	}
	if c.InputsHash == "" {
		return fmt.Errorf("claim inputs_hash required")
	}
	if c.InputsHash == "todo_inputs_hash" {
		return fmt.Errorf("claim inputs_hash placeholder")
	}
	if c.Hash == "" {
		return fmt.Errorf("claim hash required")
	}
	if !isHex64(c.Hash) {
		return fmt.Errorf("claim hash invalid")
	}
	expected, err := claimHashPayload(c)
	if err != nil {
		return err
	}
	if c.Hash != expected {
		return fmt.Errorf("claim hash mismatch")
	}
	return nil
}

func (a *AttemptV1) Validate() error {
	if a.Schema != SchemaAttemptV1 {
		return fmt.Errorf("attempt schema must be %q", SchemaAttemptV1)
	}
	if strings.TrimSpace(a.AttemptID) == "" {
		return fmt.Errorf("attempt_id required")
	}
	if strings.TrimSpace(a.ClaimHash) == "" {
		return fmt.Errorf("attempt claim_hash required")
	}
	if !isHex64(a.ClaimHash) {
		return fmt.Errorf("attempt claim_hash invalid")
	}
	if strings.TrimSpace(a.ModelID) == "" {
		return fmt.Errorf("attempt model_id required")
	}
	if strings.TrimSpace(a.OutputHash) == "" {
		return fmt.Errorf("attempt output_hash required")
	}
	if !isHex64(a.OutputHash) {
		return fmt.Errorf("attempt output_hash invalid")
	}
	if a.Timestamp <= 0 {
		return fmt.Errorf("attempt timestamp required")
	}
	if len(a.Grounding) > 0 && !json.Valid(a.Grounding) {
		return fmt.Errorf("attempt grounding invalid json")
	}
	return nil
}

func (e *EvidenceRef) Validate() error {
	if !isEvidenceKindAllowed(e.Kind) {
		return fmt.Errorf("evidence kind %q not allowed", e.Kind)
	}
	if !isHex64(e.SHA256) {
		return fmt.Errorf("evidence sha256 invalid")
	}
	return nil
}

func (p *Provenance) Validate() error {
	if strings.TrimSpace(p.Engine) == "" {
		return fmt.Errorf("provenance engine required")
	}
	if strings.TrimSpace(p.Host) == "" {
		return fmt.Errorf("provenance host required")
	}
	if p.Timestamp <= 0 {
		return fmt.Errorf("provenance timestamp required")
	}
	return nil
}

func (s *Signature) Validate() error {
	if s.Alg != SignatureAlgEd25519 {
		return fmt.Errorf("signature alg must be %q", SignatureAlgEd25519)
	}
	if strings.TrimSpace(s.PubKeyID) == "" {
		return fmt.Errorf("signature pubkey_id required")
	}
	if strings.TrimSpace(s.Sig) == "" {
		return fmt.Errorf("signature sig required")
	}
	return nil
}

func (a *AttestationV1) Validate() error {
	if a.Schema != SchemaAttestationV1 {
		return fmt.Errorf("attestation schema must be %q", SchemaAttestationV1)
	}
	if strings.TrimSpace(a.AttestationID) == "" {
		return fmt.Errorf("attestation_id required")
	}
	if strings.TrimSpace(a.ClaimHash) == "" {
		return fmt.Errorf("claim_hash required")
	}
	if !isHex64(a.ClaimHash) {
		return fmt.Errorf("claim_hash invalid")
	}
	if !isDecisionAllowed(a.Decision) {
		return fmt.Errorf("decision %q not allowed", a.Decision)
	}
	if strings.TrimSpace(a.PolicyID) == "" {
		return fmt.Errorf("policy_id required")
	}
	if len(a.EvidenceRefs) == 0 {
		return fmt.Errorf("evidence_refs required")
	}
	for i := range a.EvidenceRefs {
		if err := a.EvidenceRefs[i].Validate(); err != nil {
			return fmt.Errorf("evidence_refs[%d]: %w", i, err)
		}
	}
	if err := a.Provenance.Validate(); err != nil {
		return err
	}
	if a.Signature != nil {
		if err := a.Signature.Validate(); err != nil {
			return err
		}
	}
	return nil
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
	h, err := claimHashPayload(c)
	if err != nil {
		return err
	}
	c.Hash = h
	return nil
}

func claimHashPayload(c *ClaimV1) (string, error) {
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

	return ComputeSHA256(payload)
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

func isEvidenceKindAllowed(kind string) bool {
	switch EvidenceKind(kind) {
	case EvidenceKindL1Telemetry, EvidenceKindL2Attempt, EvidenceKindOutput, EvidenceKindEpistemicGrounding:
		return true
	default:
		return false
	}
}

func isDecisionAllowed(decision string) bool {
	switch AttestationDecision(decision) {
	case DecisionSuccess, DecisionDenied, DecisionDegraded, DecisionFail:
		return true
	default:
		return false
	}
}

func isHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// === L3: Cache ===

type L3CacheEntry struct {
	ClaimHash      string `json:"claim_hash"`
	Domain         string `json:"domain"`
	Scope          string `json:"scope"`
	AttestationRef string `json:"attestation_ref"`
	ExpiresAt      int64  `json:"expires_at"`
}
