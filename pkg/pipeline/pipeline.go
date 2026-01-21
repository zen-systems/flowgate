package pipeline

import (
	"context"
	"fmt"

	"github.com/zen-systems/flowgate/pkg/adapter"
)

// Pipeline represents a multi-stage LLM workflow.
type Pipeline struct {
	Name           string                    `yaml:"name"`
	Description    string                    `yaml:"description"`
	Workspace      Workspace                 `yaml:"workspace,omitempty"`
	DefaultAdapter string                    `yaml:"default_adapter,omitempty"`
	DefaultModel   string                    `yaml:"default_model,omitempty"`
	Gates          map[string]GateDefinition `yaml:"gates,omitempty"`
	Stages         []*Stage                  `yaml:"stages"`

	// Adapters is optional runtime configuration (not from YAML).
	Adapters map[string]adapter.Adapter `yaml:"-"`
}

// Workspace defines workspace configuration for a pipeline.
type Workspace struct {
	Path string `yaml:"path,omitempty"`
}

// GateDefinition defines a gate in the manifest.
type GateDefinition struct {
	Type         string   `yaml:"type"`
	Command      []string `yaml:"command,omitempty"`
	Workdir      string   `yaml:"workdir,omitempty"`
	BinaryPath   string   `yaml:"binary_path,omitempty"`
	ContractPath string   `yaml:"contract_path,omitempty"`
}

// Run executes the pipeline with the given input using configured adapters.
func (p *Pipeline) Run(input string) error {
	if len(p.Adapters) == 0 {
		return fmt.Errorf("pipeline adapters are not configured")
	}

	_, err := Run(context.Background(), p, RunOptions{Input: input})
	return err
}
