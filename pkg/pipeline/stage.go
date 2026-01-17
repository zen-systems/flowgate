package pipeline

// Stage represents a single step in a pipeline.
// Phase 2: Full implementation with retry and escalation.
type Stage struct {
	Name       string   `yaml:"name"`
	TaskType   string   `yaml:"task_type"`
	Adapter    string   `yaml:"adapter,omitempty"`
	Model      string   `yaml:"model,omitempty"`
	Prompt     string   `yaml:"prompt"`
	Gates      []string `yaml:"gates,omitempty"`
	MaxRetries int      `yaml:"max_retries,omitempty"`
	EscalateOn string   `yaml:"escalate_on,omitempty"`
}

// Execute runs the stage.
// Phase 2: Implements execution with gate checks and retry logic.
func (s *Stage) Execute(input string) (string, error) {
	// Stub implementation
	return "", nil
}
