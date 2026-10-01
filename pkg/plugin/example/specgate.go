package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp/go-plugin"
	flowgate_plugin "github.com/zen-systems/flowgate/pkg/plugin"
)

// SpecGatePlugin ensures code matches a specification.
type SpecGatePlugin struct {
	config map[string]interface{}
}

func NewSpecGatePlugin() *SpecGatePlugin {
	return &SpecGatePlugin{}
}

func (p *SpecGatePlugin) Manifest() flowgate_plugin.Manifest {
	return flowgate_plugin.Manifest{
		Name:    "spec-gate",
		Version: "1.0.0",
		Hooks: []flowgate_plugin.Hook{
			{Phase: flowgate_plugin.PhaseVerification, Priority: 100},
		},
		Capabilities: []string{"read:workspace"},
	}
}

func (p *SpecGatePlugin) Init(config map[string]interface{}) error {
	p.config = config
	return nil
}

func (p *SpecGatePlugin) Analyze(ctx context.Context, req flowgate_plugin.RequestContext) (*flowgate_plugin.Result, error) {
	// 1. Check for required artifacts
	spec, ok := req.Artifacts["spec.md"]
	if !ok {
		return &flowgate_plugin.Result{
			Status: flowgate_plugin.StatusSkip,
			Diagnostics: []flowgate_plugin.Diagnostic{
				{Severity: "warning", Message: "No spec.md found, skipping gate"},
			},
		}, nil
	}
	code, ok := req.Artifacts["main.go"] // Simplified hook
	if !ok {
		return &flowgate_plugin.Result{
			Status: flowgate_plugin.StatusFail,
			Diagnostics: []flowgate_plugin.Diagnostic{
				{Severity: "error", Message: "No main.go found to verify against spec"},
			},
		}, nil
	}

	// 2. Perform verification logic (simplified simulation)
	// In reality, this would call an LLM or static analysis tool.
	// For this example, we assume if both exist, it passes, but we hash them for the attestation.

	claim := map[string]interface{}{
		"spec_hash": hash(spec),
		"code_hash": hash(code),
		"verified":  true,
		"method":    "heuristic_existence_check", // Mock method
	}

	attestation := flowgate_plugin.Attestation{
		Subject: "main.go",
		Claim:   claim,
		Hash:    hashString(fmt.Sprintf("%v", claim)),
		Signer:  "spec-gate-v1",
	}

	return &flowgate_plugin.Result{
		Status:       flowgate_plugin.StatusPass,
		Attestations: []flowgate_plugin.Attestation{attestation},
		Diagnostics: []flowgate_plugin.Diagnostic{
			{Severity: "info", Message: "Spec and Code verified"},
		},
	}, nil
}

func (p *SpecGatePlugin) Shutdown() error {
	return nil
}

func hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hashString(s string) string {
	return hash([]byte(s))
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: flowgate_plugin.Handshake,
		Plugins: map[string]plugin.Plugin{
			"flowgate_plugin": &flowgate_plugin.FlowgatePluginImpl{Impl: NewSpecGatePlugin()},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
