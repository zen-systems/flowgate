package pipeline

import "fmt"

// Stage represents a single step in a pipeline.
type Stage struct {
	Name       string   `yaml:"name"`
	TaskType   string   `yaml:"task_type"`
	Adapter    string   `yaml:"adapter,omitempty"`
	Model      string   `yaml:"model,omitempty"`
	Prompt     string   `yaml:"prompt"`
	Gates      []string `yaml:"gates,omitempty"`
	MaxRetries int      `yaml:"max_retries,omitempty"`
	Apply      bool     `yaml:"apply,omitempty"`
	EscalateOn string   `yaml:"escalate_on,omitempty"`
}

// Execute runs a stage directly. Prefer running via the pipeline runner.
func (s *Stage) Execute(_ string) (string, error) {
	return "", fmt.Errorf("stage execution requires pipeline runner")
}
