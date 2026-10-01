package epistemic

// EvidenceRef points to evidence used in a decision.
type EvidenceRef struct {
	Source  string `json:"source"`
	Query   string `json:"query,omitempty"`
	Title   string `json:"title,omitempty"`
	URL     string `json:"url,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// RoutingDecision captures heuristic and final decisions.
type RoutingDecision struct {
	Heuristic Decision `json:"heuristic"`
	Final     Decision `json:"final"`
}

// Decision captures an auditable routing decision.
type Decision struct {
	Kind         string        `json:"kind"`
	Candidates   []string      `json:"candidates,omitempty"`
	Choice       string        `json:"choice"`
	Confidence   float64       `json:"confidence"`
	Reasons      []string      `json:"reasons,omitempty"`
	NeedsEgress  bool          `json:"needs_egress"`
	Impact       string        `json:"impact"`
	Escalated    bool          `json:"escalated"`
	EvidenceUsed []EvidenceRef `json:"evidence_used,omitempty"`
}

// EpistemicPolicy decides when to escalate.
type EpistemicPolicy struct {
	Enable              bool    `json:"enable"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
}

// ShouldEscalate decides whether to escalate a decision.
func (p EpistemicPolicy) ShouldEscalate(decision Decision) bool {
	if !p.Enable {
		return false
	}
	if !decision.NeedsEgress {
		return false
	}
	if p.ConfidenceThreshold <= 0 {
		return false
	}
	if decision.Confidence >= p.ConfidenceThreshold {
		return false
	}
	if len(decision.Candidates) <= 1 {
		return false
	}
	return true
}
