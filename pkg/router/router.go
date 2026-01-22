package router

import (
	"context"
	"fmt"
	"log"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/config"
)

// Router defines the interface for routing prompts to adapters.
type Router interface {
	// Route determines which adapter and model to use for a prompt.
	Route(prompt string) (adapter.Adapter, string)

	// Send routes and sends a prompt, returning the result.
	Send(ctx context.Context, prompt string) (*adapter.Response, error)

	// GetAdapter returns an adapter by name.
	GetAdapter(name string) (adapter.Adapter, bool)

	// GetRoutes returns all configured routing rules.
	GetRoutes() []RouteInfo
}

// RouteInfo describes a routing rule.
type RouteInfo struct {
	TaskType      string
	Triggers      []string
	Adapter       string
	Model         string // May be alias
	ResolvedModel string // Canonical model name
}

// DefaultRouter implements Router using pattern matching.
type DefaultRouter struct {
	adapters map[string]adapter.Adapter
	aliases  *config.ModelAliases
	rules    *RuleSet
	config   *config.RoutingConfig
	debug    bool
	decision *Decision
}

// RouterOption configures a DefaultRouter.
type RouterOption func(*DefaultRouter)

// WithAliases sets the model aliases for the router.
func WithAliases(aliases *config.ModelAliases) RouterOption {
	return func(r *DefaultRouter) {
		r.aliases = aliases
	}
}

// WithDebug enables debug logging.
func WithDebug(debug bool) RouterOption {
	return func(r *DefaultRouter) {
		r.debug = debug
	}
}

// NewRouter creates a new router with the given adapters and routing config.
func NewRouter(adapters map[string]adapter.Adapter, cfg *config.RoutingConfig, opts ...RouterOption) *DefaultRouter {
	rules := NewRuleSet(cfg)
	r := &DefaultRouter{
		adapters: adapters,
		rules:    rules,
		config:   cfg,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Route determines the adapter and model for a prompt.
func (r *DefaultRouter) Route(prompt string) (adapter.Adapter, string) {
	a, model, _ := r.RouteWithDecision(context.Background(), prompt)
	return a, model
}

// RouteWithDecision determines the adapter/model for a prompt and returns the decision.
func (r *DefaultRouter) RouteWithDecision(ctx context.Context, prompt string) (adapter.Adapter, string, *Decision) {
	adapterName, model := r.rules.Match(prompt)
	decision, err := NewClassifier(r.adapters, r.config).Classify(ctx, prompt)
	if err != nil && r.debug {
		log.Printf("[router] classifier error: %v", err)
	}
	if decision != nil && decision.TaskType != "" && decision.TaskType != "default" {
		if task, ok := r.config.TaskTypes[decision.TaskType]; ok {
			adapterName = task.Adapter
			model = task.Model
		}
	}

	a, ok := r.adapters[adapterName]
	if !ok {
		a = r.adapters[r.config.Default.Adapter]
		model = r.config.Default.Model
	}

	// Resolve alias to canonical model name
	resolvedModel := r.resolveModel(model)

	if r.debug && model != resolvedModel {
		log.Printf("[router] resolved alias %q -> %q", model, resolvedModel)
	}
	if decision != nil {
		r.decision = decision
	}

	return a, resolvedModel, decision
}

// resolveModel resolves a model alias to its canonical name.
func (r *DefaultRouter) resolveModel(model string) string {
	if r.aliases != nil {
		return r.aliases.Resolve(model)
	}
	return model
}

// Send routes the prompt and generates a response.
func (r *DefaultRouter) Send(ctx context.Context, prompt string) (*adapter.Response, error) {
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
			TaskType:      name,
			Triggers:      taskType.Triggers,
			Adapter:       taskType.Adapter,
			Model:         taskType.Model,
			ResolvedModel: r.resolveModel(taskType.Model),
		})
	}
	return routes
}

// GetAliases returns the model aliases, if configured.
func (r *DefaultRouter) GetAliases() *config.ModelAliases {
	return r.aliases
}

// Decision returns the last routing decision, if any.
func (r *DefaultRouter) Decision() *Decision {
	return r.decision
}
