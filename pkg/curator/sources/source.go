// Package sources provides information sources for the curator.
package sources

import (
	"context"
	"time"
)

// QueryResult represents information retrieved from a source.
type QueryResult struct {
	Content    string            // The information content
	Path       string            // Source path, URL, or reference
	Confidence float64           // 0-1 confidence this answers the query
	Timestamp  time.Time         // When the source was created/modified
	Metadata   map[string]string // Additional context
}

// Source defines the interface for information sources.
type Source interface {
	// Name returns the source identifier.
	Name() string

	// Query searches the source for information matching the query.
	Query(ctx context.Context, query string) ([]QueryResult, error)

	// Available returns true if the source is currently available.
	Available() bool
}

// SourceRegistry holds all available sources.
type SourceRegistry struct {
	sources map[string]Source
}

// NewRegistry creates a new source registry.
func NewRegistry() *SourceRegistry {
	return &SourceRegistry{
		sources: make(map[string]Source),
	}
}

// Register adds a source to the registry.
func (r *SourceRegistry) Register(source Source) {
	r.sources[source.Name()] = source
}

// Get returns a source by name.
func (r *SourceRegistry) Get(name string) (Source, bool) {
	s, ok := r.sources[name]
	return s, ok
}

// Available returns all available sources.
func (r *SourceRegistry) Available() []Source {
	var available []Source
	for _, s := range r.sources {
		if s.Available() {
			available = append(available, s)
		}
	}
	return available
}

// All returns all registered sources.
func (r *SourceRegistry) All() []Source {
	sources := make([]Source, 0, len(r.sources))
	for _, s := range r.sources {
		sources = append(sources, s)
	}
	return sources
}
