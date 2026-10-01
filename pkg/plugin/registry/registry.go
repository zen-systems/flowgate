package registry

import (
	"fmt"
	"sync"

	"github.com/zen-systems/flowgate/pkg/plugin"
)

// Registry manages the lifecycle and lookup of plugins.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]plugin.Plugin
	byPhase map[plugin.Phase][]plugin.Plugin
}

// New creates a new plugin registry.
func New() *Registry {
	return &Registry{
		plugins: make(map[string]plugin.Plugin),
		byPhase: make(map[plugin.Phase][]plugin.Plugin),
	}
}

// Register adds a plugin to the registry.
// Ideally, this is where signature verification would happen.
func (r *Registry) Register(p plugin.Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	manifest := p.Manifest()
	if _, exists := r.plugins[manifest.Name]; exists {
		return fmt.Errorf("plugin %s already registered", manifest.Name)
	}

	// 1. Verify compatibility (mock)
	if manifest.Version == "" {
		return fmt.Errorf("plugin %s missing version", manifest.Name)
	}

	// 2. Register Hooks
	for _, hook := range manifest.Hooks {
		r.byPhase[hook.Phase] = append(r.byPhase[hook.Phase], p)
		// Note: In a real impl, we would sort by priority here.
	}

	r.plugins[manifest.Name] = p
	return nil
}

// GetPluginsForPhase returns all plugins registered for a specific phase.
func (r *Registry) GetPluginsForPhase(phase plugin.Phase) []plugin.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent mutation of the slice
	plugins := r.byPhase[phase]
	result := make([]plugin.Plugin, len(plugins))
	copy(result, plugins)

	return result
}

// ShutdownAll gracefully shuts down all plugins.
func (r *Registry) ShutdownAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, p := range r.plugins {
		if err := p.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s shutdown failed: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
