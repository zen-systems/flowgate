package pipeline

// Pipeline represents a multi-stage LLM workflow.
// Phase 2: Full implementation.
type Pipeline struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Stages      []*Stage `yaml:"stages"`
}

// Run executes the pipeline with the given input.
// Phase 2: Implements stage execution with retry and escalation.
func (p *Pipeline) Run(input string) error {
	// Stub implementation
	return nil
}
