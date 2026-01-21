package pipeline

import (
	"os"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	content := `name: test-pipeline
description: test

gates:
  go_test:
    type: command
    command: ["echo", "ok"]

stages:
  - name: stage1
    adapter: mock
    model: mock-1
    prompt: "Hello {{ .Input }}"
    gates:
      - go_test
`

	file, err := os.CreateTemp("", "pipeline-*.yaml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	file.Close()

	p, err := LoadManifest(file.Name())
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
