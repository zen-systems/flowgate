package router

import (
	"context"
	"fmt"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/config"
)

// Router defines the interface for routing prompts to adapters.
type Router interface {
	// Route determines which adapter and model to use for a prompt.
	Route(prompt string) (adapter.Adapter, string)

	// Send routes and sends a prompt, returning the result.
	Send(ctx context.Context, prompt string) (*artifact.Artifact, error)

	// GetAdapter returns an adapter by name.
	GetAdapter(name string) (adapter.Adapter, bool)

	// GetRoutes returns all configured routing rules.
	GetRoutes() []RouteInfo
}

// RouteInfo describes a routing rule.
type RouteInfo struct {
	TaskType string
	Triggers []string
	Adapter  string
	Model    string
}

// DefaultRouter implements Router using pattern matching.
type DefaultRouter struct {
	adapters map[string]adapter.Adapter
	rules    *RuleSet
	config   *config.RoutingConfig
}

// NewRouter creates a new router with the given adapters and routing config.
func NewRouter(adapters map[string]adapter.Adapter, cfg *config.RoutingConfig) *DefaultRouter {
	rules := NewRuleSet(cfg)
	return &DefaultRouter{
		adapters: adapters,
		rules:    rules,
		config:   cfg,
	}
}

// Route determines the adapter and model for a prompt.
func (r *DefaultRouter) Route(prompt string) (adapter.Adapter, string) {
	adapterName, model := r.rules.Match(prompt)

	a, ok := r.adapters[adapterName]
	if !ok {
		a = r.adapters[r.config.Default.Adapter]
		model = r.config.Default.Model
	}

	return a, model
}

// Send routes the prompt and generates a response.
func (r *DefaultRouter) Send(ctx context.Context, prompt string) (*artifact.Artifact, error) {
	a, model := r.Route(prompt)
	if a == nil {
		return nil, fmt.Errorf("no adapter available for prompt")
	}

	return a.Generate(ctx, model, prompt)
}

// GetAdapter returns an adapter by name.
func (r *DefaultRouter) GetAdapter(name string) (adapter.Adapter, bool) {
	a, ok := r.adapters[name]
	return a, ok
}

// GetRoutes returns all configured routing rules.
func (r *DefaultRouter) GetRoutes() []RouteInfo {
	var routes []RouteInfo
	for name, taskType := range r.config.TaskTypes {
		routes = append(routes, RouteInfo{
			TaskType: name,
			Triggers: taskType.Triggers,
			Adapter:  taskType.Adapter,
			Model:    taskType.Model,
		})
	}
	return routes
}
