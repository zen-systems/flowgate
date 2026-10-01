package plugin_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zen-systems/flowgate/pkg/plugin"
)

func TestPluginArchitecture_EndToEnd(t *testing.T) {
	// 1. Compile the example plugin
	pluginPath := filepath.Join(os.TempDir(), "flowgate-spec-gate-plugin")
	// Clean up previous build
	os.Remove(pluginPath)

	// Ensure cleanup after test
	defer os.Remove(pluginPath)

	wd, _ := os.Getwd()
	// Try resolve absolute path assuming project root
	pluginSource := filepath.Join(wd, "pkg/plugin/example/specgate.go")
	if _, err := os.Stat(pluginSource); os.IsNotExist(err) {
		// Fallback: assume running from pkg/plugin directory
		pluginSource = filepath.Join(wd, "example/specgate.go")
	}

	cmd := exec.Command("go", "build", "-o", pluginPath, pluginSource)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to compile example plugin: %v\nOutput: %s", err, out)
	}

	// 2. Load the plugin
	loader := plugin.NewLoader(pluginPath)
	defer loader.Close()

	p, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}

	// 3. Verify Manifest
	// NOTE: In current simplified gRPC impl, Manifest is not over RPC yet, but normally would be.
	// The loader just returns the GRPCClient.
	// Let's assume for this test we are testing Analyze capability.

	// 4. Test Analyze
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Case 1: Missing spec.md (Should Skip)
	req := plugin.RequestContext{
		RunID:     "test-run-1",
		Input:     "verifyme",
		Artifacts: map[string][]byte{},
	}
	res, err := p.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if res.Status != plugin.StatusSkip {
		t.Errorf("Expected StatusSkip for missing spec, got %v", res.Status)
	}

	// Case 2: Success (Spec + Code)
	req.Artifacts["spec.md"] = []byte("# Spec")
	req.Artifacts["main.go"] = []byte("package main")

	res, err = p.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if res.Status != plugin.StatusPass {
		t.Errorf("Expected StatusPass, got %v", res.Status)
	}
	if len(res.Attestations) == 0 {
		t.Error("Expected attestations, got 0")
	} else {
		t.Logf("Received Attestation: %+v", res.Attestations[0])
	}
}
