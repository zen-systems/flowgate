package pipeline

import "testing"

func TestRenderPromptWithArtifacts(t *testing.T) {
	artifacts := map[string]ArtifactTemplateData{
		"build": {Text: "artifact text", Output: "artifact text", Hash: "abc"},
	}
	stages := map[string]map[string]string{
		"build": {"output": "legacy output"},
	}

	prompt, err := renderPrompt("Input: {{ .Input }} | {{ .Artifacts.build.Text }} | {{ .stages.build.output }}", "hello", artifacts, stages)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	expected := "Input: hello | artifact text | legacy output"
	if prompt != expected {
		t.Fatalf("unexpected prompt: %q", prompt)
	}
}
