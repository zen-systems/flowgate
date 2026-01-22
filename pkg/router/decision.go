package router

// Candidate captures a heuristic candidate task type.
type Candidate struct {
	TaskType string   `json:"task_type"`
	Score    int      `json:"score"`
	Triggers []string `json:"triggers,omitempty"`
	Adapter  string   `json:"adapter,omitempty"`
	Model    string   `json:"model,omitempty"`
}

// RoutingFeedback captures post-run routing feedback.
type RoutingFeedback struct {
	GatesPassed    bool   `json:"gates_passed"`
	AttemptsNeeded int    `json:"attempts_needed"`
	WouldReroute   string `json:"would_reroute,omitempty"`
}

// Decision captures routing decision details.
type Decision struct {
	TaskType          string           `json:"task_type"`
	Confidence        float64          `json:"confidence"`
	Reasons           []string         `json:"reasons,omitempty"`
	Candidates        []Candidate      `json:"candidates,omitempty"`
	UsedLLM           bool             `json:"used_llm"`
	ClassifierAdapter string           `json:"classifier_adapter,omitempty"`
	ClassifierModel   string           `json:"classifier_model,omitempty"`
	Feedback          *RoutingFeedback `json:"post_run_feedback,omitempty"`
}
