package curator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zen-systems/flowgate/pkg/curator/sources"
)

// Gatherer collects information from multiple sources in parallel.
type Gatherer struct {
	registry      *sources.SourceRegistry
	maxParallel   int
	maxPerSource  int
	timeout       time.Duration
}

// GathererOption configures a Gatherer.
type GathererOption func(*Gatherer)

// WithMaxParallel sets the maximum parallel gather operations.
func WithMaxParallel(max int) GathererOption {
	return func(g *Gatherer) {
		g.maxParallel = max
	}
}

// WithMaxPerSource sets the maximum results per source.
func WithMaxPerSource(max int) GathererOption {
	return func(g *Gatherer) {
		g.maxPerSource = max
	}
}

// WithGatherTimeout sets the timeout for gather operations.
func WithGatherTimeout(timeout time.Duration) GathererOption {
	return func(g *Gatherer) {
		g.timeout = timeout
	}
}

// NewGatherer creates a new Gatherer.
func NewGatherer(registry *sources.SourceRegistry, opts ...GathererOption) *Gatherer {
	g := &Gatherer{
		registry:     registry,
		maxParallel:  10,
		maxPerSource: 20,
		timeout:      30 * time.Second,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// gatherTask represents a single gather operation.
type gatherTask struct {
	need       InformationNeed
	sourceType SourceType
}

// gatherResult holds the result of a gather operation.
type gatherResult struct {
	info []GatheredInfo
	err  error
}

// Gather collects information for all needs from their suggested sources.
func (g *Gatherer) Gather(ctx context.Context, needs []InformationNeed) ([]GatheredInfo, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Build list of tasks
	var tasks []gatherTask
	for _, need := range needs {
		for _, sourceType := range need.Sources {
			tasks = append(tasks, gatherTask{
				need:       need,
				sourceType: sourceType,
			})
		}
	}

	// Process tasks with bounded parallelism
	results := make(chan gatherResult, len(tasks))
	semaphore := make(chan struct{}, g.maxParallel)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t gatherTask) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- gatherResult{err: ctx.Err()}
				return
			}

			// Execute gather
			info, err := g.gatherFromSource(ctx, t.need, t.sourceType)
			results <- gatherResult{info: info, err: err}
		}(task)
	}

	// Close results channel when all tasks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect all results
	var allInfo []GatheredInfo
	var errors []error

	for result := range results {
		if result.err != nil {
			errors = append(errors, result.err)
			continue
		}
		allInfo = append(allInfo, result.info...)
	}

	// Return error only if all tasks failed
	if len(allInfo) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all gather operations failed: %v", errors[0])
	}

	return allInfo, nil
}

// gatherFromSource queries a single source for a single need.
func (g *Gatherer) gatherFromSource(ctx context.Context, need InformationNeed, sourceType SourceType) ([]GatheredInfo, error) {
	source, ok := g.registry.Get(string(sourceType))
	if !ok {
		return nil, fmt.Errorf("source %s not found", sourceType)
	}

	if !source.Available() {
		return nil, fmt.Errorf("source %s not available", sourceType)
	}

	results, err := source.Query(ctx, need.Query)
	if err != nil {
		return nil, fmt.Errorf("query to %s failed: %w", sourceType, err)
	}

	// Convert to GatheredInfo and limit results
	info := make([]GatheredInfo, 0, g.maxPerSource)
	for i, result := range results {
		if i >= g.maxPerSource {
			break
		}

		info = append(info, GatheredInfo{
			ID:         fmt.Sprintf("gathered_%s_%d", need.ID, i+1),
			NeedID:     need.ID,
			Content:    result.Content,
			Source:     sourceType,
			SourcePath: result.Path,
			Confidence: result.Confidence,
			Timestamp:  result.Timestamp,
			Metadata:   result.Metadata,
		})
	}

	return info, nil
}

// GatherForNeed collects information for a single need from all its sources.
func (g *Gatherer) GatherForNeed(ctx context.Context, need InformationNeed) ([]GatheredInfo, error) {
	return g.Gather(ctx, []InformationNeed{need})
}

// GatherFromSources collects information from specific sources only.
func (g *Gatherer) GatherFromSources(ctx context.Context, query string, sourceTypes []SourceType) ([]GatheredInfo, error) {
	need := InformationNeed{
		ID:       "manual_query",
		Query:    query,
		Type:     InfoTypeContext,
		Required: true,
		Sources:  sourceTypes,
		Priority: 5,
	}
	return g.Gather(ctx, []InformationNeed{need})
}
