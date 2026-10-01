package epistemic

import "testing"

func TestEpistemicPolicyShouldEscalate(t *testing.T) {
	policy := EpistemicPolicy{Enable: true, ConfidenceThreshold: 0.7}
	decision := Decision{NeedsEgress: true, Confidence: 0.5, Candidates: []string{"a", "b"}}
	if !policy.ShouldEscalate(decision) {
		t.Fatalf("expected escalation")
	}

	decision = Decision{NeedsEgress: false, Confidence: 0.5, Candidates: []string{"a", "b"}}
	if policy.ShouldEscalate(decision) {
		t.Fatalf("expected no escalation when egress not needed")
	}

	policy = EpistemicPolicy{Enable: false, ConfidenceThreshold: 0.7}
	decision = Decision{NeedsEgress: true, Confidence: 0.2, Candidates: []string{"a", "b"}}
	if policy.ShouldEscalate(decision) {
		t.Fatalf("expected no escalation when policy disabled")
	}
}
