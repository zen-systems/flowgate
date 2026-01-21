package pipeline

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadManifest reads a pipeline definition from a YAML file.
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
func (p *Pipeline) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("pipeline must define at least one stage")
	}

	seen := make(map[string]struct{})
	for _, stage := range p.Stages {
		if stage.Name == "" {
			return fmt.Errorf("stage name is required")
		}
		if stage.Prompt == "" {
			return fmt.Errorf("stage %s must have a prompt", stage.Name)
		}
		if _, ok := seen[stage.Name]; ok {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		seen[stage.Name] = struct{}{}

		for _, gateName := range stage.Gates {
			if gateName == "" {
				return fmt.Errorf("stage %s has empty gate name", stage.Name)
			}
			if _, ok := p.Gates[gateName]; !ok && gateName != "hollowcheck" {
				return fmt.Errorf("stage %s references unknown gate %s", stage.Name, gateName)
			}
		}
	}

	return nil
}
