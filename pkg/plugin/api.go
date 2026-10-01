package plugin

import (
	"context"
)

// Phase defines where a plugin hooks into the pipeline.
type Phase string

const (
	PhaseIntent       Phase = "intent"
	PhaseRouting      Phase = "routing"
	PhaseVerification Phase = "verification"
	PhaseReasoning    Phase = "reasoning"
	PhaseGovernance   Phase = "governance"
)

// Manifest describes the plugin's metadata and requirements.
type Manifest struct {
	Name         string   `yaml:"name" json:"name"`
	Version      string   `yaml:"version" json:"version"`
	Hooks        []Hook   `yaml:"hooks" json:"hooks"`
	Capabilities []string `yaml:"capabilities" json:"capabilities"`
}

// Hook defines a specific integration point.
type Hook struct {
	Phase    Phase `yaml:"phase" json:"phase"`
	Priority int   `yaml:"priority" json:"priority"` // Higher runs first
}

// RequestContext provides read-only access to the current system state.
// Plugins must NOT mutate this directly.
type RequestContext struct {
	RunID       string
	Input       string
	Artifacts   map[string][]byte
	Metadata    map[string]string
	Permissions []string
}

// Result captures the output of a plugin execution.
type Result struct {
	// Status indicates the high-level outcome.
	Status Status

	// Artifacts to add to the run context (new analysis, verified code, etc).
	Artifacts map[string][]byte

	// Attestations providing cryptographic proof of the plugin's work.
	Attestations []Attestation

	// Diagnostics for logging and debugging.
	Diagnostics []Diagnostic
}

// Status represents the judgment of the plugin.
type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusNeutral Status = "neutral" // Informational only
	StatusSkip    Status = "skip"
)

// Attestation is a formal claim made by the plugin.
type Attestation struct {
	Subject string
	Claim   map[string]interface{}
	Hash    string // SHA-256 of the claim content
	Signer  string // Plugin ID
}

// Diagnostic represents a log or error.
type Diagnostic struct {
	Severity string // "info", "warning", "error"
	Message  string
	Context  map[string]interface{}
}

// Plugin is the primary interface for all Flowgate extensions.
// In a remote setting (gRPC), this interface is adapted.
type Plugin interface {
	// Manifest returns the static metadata for the plugin.
	Manifest() Manifest

	// Init initializes the plugin.
	Init(config map[string]interface{}) error

	// Analyze performs the main logic for a given phase.
	Analyze(ctx context.Context, req RequestContext) (*Result, error)

	// Shutdown cleans up resources.
	Shutdown() error
}
