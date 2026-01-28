package policy

import "fmt"

type Requirements struct {
	MaxEntropy         float32
	RequireHollowcheck bool
	MinComplexity      float32
}

type Policy struct {
	ID           string
	Requirements Requirements
}

type Registry struct {
	policies map[string]Policy
}

func NewRegistry() *Registry {
	r := &Registry{
		policies: make(map[string]Policy),
	}

	// Register default policies
	r.Register(Policy{
		ID: "policy_v1_strict_no_stubs",
		Requirements: Requirements{
			MaxEntropy:         4.5,
			RequireHollowcheck: true,
			MinComplexity:      0.4,
		},
	})

	r.Register(Policy{
		ID: "policy_v1_permissive",
		Requirements: Requirements{
			MaxEntropy:         10.0,
			RequireHollowcheck: false,
			MinComplexity:      0.0,
		},
	})

	return r
}

func (r *Registry) Register(p Policy) {
	r.policies[p.ID] = p
}

func (r *Registry) Get(id string) (Policy, error) {
	p, ok := r.policies[id]
	if !ok {
		return Policy{}, fmt.Errorf("policy not found: %s", id)
	}
	return p, nil
}
