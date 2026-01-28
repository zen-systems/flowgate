# Promotion Path Validation Rules

This note documents validation enforced for L4 promotion.

## Required For Promotion
- EvaluateResponse.Evidence must parse as EpistemicEvidence with Claims.ClaimsVerified > 0.
- ClaimV1 schema `vtp.claim.v1` with non-empty content/domain, inputs_hash set (not "todo_inputs_hash"), and claim_hash matching computed hash.
- InputsHash must not be `inputs.unknown` under policy `policy_v1_strict_no_stubs`.
- AttemptV1 schema `flowgate.l2.attempt.v1` with non-empty claim_hash/model_id/output_hash and valid grounding JSON when present.
- EvidenceRefs must include stored l2 attempts and grounding; all required store calls must succeed.

## Fatal Errors (Promotion Stops)
- Any required evidence/attestation storage failure.
- Invalid schema or enum values for ClaimV1, AttemptV1, AttestationV1, EvidenceRef, or Provenance.
- Invalid hashes on claim, attempt output, or evidence refs.
- Missing attestation signature at storage time.

## Allowed Enums
- Attestation decisions: DecisionSuccess, DecisionDenied, DecisionDegraded (DecisionFail accepted as deprecated alias).
- EvidenceRef.Kind: l1.telemetry, l2.attempt, output, epistemic.grounding.
