package pipeline

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadManifest reads a pipeline definition from a YAML file.
// Phase 2: Full implementation with validation.
func LoadManifest(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pipeline Pipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, err
	}

	return &pipeline, nil
}

// Validate checks the pipeline configuration for errors.
// Phase 2: Implements comprehensive validation.
func (p *Pipeline) Validate() error {
	// Stub implementation
	return nil
}
