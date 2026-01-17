package curator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zen-systems/flowgate/pkg/adapter"
	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/curator/sources"
)

// Curator is a meta-adapter that orchestrates context gathering before responding.
// It implements the adapter.Adapter interface.
type Curator struct {
	config        CuratorConfig
	targetAdapter adapter.Adapter
	analyzer      *Analyzer
	gatherer      *Gatherer
	organizer     *Organizer
	reconciler    *Reconciler
	decider       *Decider
	synthesizer   *Synthesizer
	registry      *sources.SourceRegistry

	// Observable logging
	logger func(format string, args ...any)
}

// CuratorOption configures a Curator.
type CuratorOption func(*Curator)

// WithLogger sets a custom logger for observable output.
func WithLogger(logger func(format string, args ...any)) CuratorOption {
	return func(c *Curator) {
		c.logger = logger
	}
}

// WithConfig overrides the default configuration.
func WithConfig(cfg CuratorConfig) CuratorOption {
	return func(c *Curator) {
		c.config = cfg
	}
}

// WithMemorySource adds a memory source to the curator.
func WithMemorySource(mem *sources.MemorySource) CuratorOption {
	return func(c *Curator) {
		c.registry.Register(mem)
	}
}

// WithArtifactSource adds an artifact source to the curator.
func WithArtifactSource(art *sources.ArtifactSource) CuratorOption {
	return func(c *Curator) {
		c.registry.Register(art)
	}
}

// WithFilesystemSource adds a filesystem source with the given base path.
func WithFilesystemSource(basePath string, opts ...sources.FilesystemOption) CuratorOption {
	return func(c *Curator) {
		fs := sources.NewFilesystemSource(basePath, opts...)
		c.registry.Register(fs)
	}
}

// WithWebSource adds a Tavily-backed web search source.
// Requires TAVILY_API_KEY environment variable or use WithTavilyAPIKey option.
func WithWebSource(opts ...sources.TavilyOption) CuratorOption {
	return func(c *Curator) {
		web := sources.NewWebSource(opts...)
		if web.Available() {
			c.registry.Register(web)
		}
	}
}

// NewCurator creates a new Curator with the given target adapter.
func NewCurator(targetAdapter adapter.Adapter, analysisAdapter adapter.Adapter, opts ...CuratorOption) (*Curator, error) {
	if targetAdapter == nil {
		return nil, fmt.Errorf("target adapter is required")
	}
	if analysisAdapter == nil {
		analysisAdapter = targetAdapter
	}

	cfg := DefaultConfig()
	registry := sources.NewRegistry()

	c := &Curator{
		config:        cfg,
		targetAdapter: targetAdapter,
		registry:      registry,
		logger:        defaultLogger,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize stages
	c.analyzer = NewAnalyzer(analysisAdapter, c.config.AnalysisModel)
	c.gatherer = NewGatherer(
		registry,
		WithMaxParallel(c.config.MaxGatherParallel),
		WithMaxPerSource(c.config.MaxGatherPerSource),
	)
	c.organizer = NewOrganizer(analysisAdapter, c.config.AnalysisModel, c.config.ContextBudget)
	c.reconciler = NewReconciler(analysisAdapter, c.config.AnalysisModel)
	c.decider = NewDecider(c.config.ConfidenceThreshold, 0.8)
	c.synthesizer = NewSynthesizer(analysisAdapter, c.config.AnalysisModel, c.config.ContextBudget)

	return c, nil
}

// Name returns the adapter identifier.
func (c *Curator) Name() string {
	return "curator"
}

// Models returns the supported models (delegates to target adapter).
func (c *Curator) Models() []string {
	return c.targetAdapter.Models()
}

// Generate orchestrates context curation and generates a response.
func (c *Curator) Generate(ctx context.Context, model string, prompt string) (*artifact.Artifact, error) {
	startTime := time.Now()
	result := &CuratedContext{
		Query:  prompt,
		Stages: make([]StageLog, 0),
	}

	// Stage 1: Analyze
	c.log("[curator] Analyzing query...")
	stageStart := time.Now()

	needs, err := c.analyzer.Analyze(ctx, prompt)
	if err != nil {
		// Fall back to simple analysis
		c.log("[curator] Model analysis failed, using simple analysis")
		needs = c.analyzer.AnalyzeSimple(prompt)
	}

	result.Needs = needs
	result.Stages = append(result.Stages, StageLog{
		Stage:     "analyze",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     len(needs),
		Notes:     fmt.Sprintf("Identified %d information needs", len(needs)),
	})
	c.log("[curator] Identified %d information needs", len(needs))

	// Stage 2: Gather
	c.log("[curator] Gathering from %d sources...", c.countActiveSources())
	stageStart = time.Now()

	gathered, err := c.gatherer.Gather(ctx, needs)
	if err != nil {
		c.log("[curator] Warning: gather error: %v", err)
	}

	result.Gathered = gathered
	result.Stages = append(result.Stages, StageLog{
		Stage:     "gather",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     len(gathered),
		Notes:     fmt.Sprintf("Gathered %d items", len(gathered)),
	})
	c.log("[curator] Gathered %d items", len(gathered))

	// Stage 3: Organize
	c.log("[curator] Organizing and ranking...")
	stageStart = time.Now()

	organized, err := c.organizer.Organize(ctx, prompt, needs, gathered)
	if err != nil {
		organized = gathered // Use unorganized on error
	}
	organized = DeduplicateByContent(organized)

	result.Organized = organized
	result.Stages = append(result.Stages, StageLog{
		Stage:     "organize",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     len(organized),
		Notes:     fmt.Sprintf("Retained %d relevant items", len(organized)),
	})
	c.log("[curator] Retained %d relevant items", len(organized))

	// Stage 4: Reconcile
	c.log("[curator] Checking for conflicts and gaps...")
	stageStart = time.Now()

	conflicts, gaps, err := c.reconciler.Reconcile(ctx, needs, organized)
	if err != nil {
		c.log("[curator] Warning: reconcile error: %v", err)
	}

	result.Conflicts = conflicts
	result.Gaps = gaps
	result.Stages = append(result.Stages, StageLog{
		Stage:     "reconcile",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     len(conflicts) + len(gaps),
		Notes:     fmt.Sprintf("Found %d conflicts, %d gaps", len(conflicts), len(gaps)),
	})

	for _, conflict := range conflicts {
		if conflict.Resolved {
			c.log("[curator] Conflict resolved: %s", conflict.Resolution)
		} else {
			c.log("[curator] Unresolved conflict: %s", conflict.Topic)
		}
	}
	if len(gaps) > 0 {
		c.log("[curator] Gap: %s", gaps[0].Reason)
	}

	// Stage 5: Decide
	stageStart = time.Now()
	decision := c.decider.Decide(needs, organized, gaps)
	result.CanAnswer = decision.CanAnswer
	result.ClarifyingQs = decision.ClarifyingQs

	result.Stages = append(result.Stages, StageLog{
		Stage:     "decide",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     1,
		Notes:     fmt.Sprintf("Can answer: %v, confidence: %.0f%%", decision.CanAnswer, decision.Confidence*100),
	})

	// If we can't answer, return clarifying questions
	if !decision.CanAnswer && len(decision.ClarifyingQs) > 0 {
		c.log("[curator] Cannot answer - returning clarifying questions")
		return c.createClarifyingResponse(decision.ClarifyingQs, model, prompt)
	}

	// Stage 6: Synthesize
	c.log("[curator] Synthesizing context...")
	stageStart = time.Now()

	curatedContext, tokenCount, err := c.synthesizer.Synthesize(ctx, prompt, organized, conflicts)
	if err != nil {
		curatedContext = c.synthesizer.SynthesizeSimple(organized, conflicts)
		tokenCount = estimateTokens(curatedContext)
	}

	result.FinalContext = curatedContext
	result.TokenEstimate = tokenCount
	result.Stages = append(result.Stages, StageLog{
		Stage:     "synthesize",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     tokenCount,
		Notes:     fmt.Sprintf("Context: %d tokens", tokenCount),
	})
	c.log("[curator] Curated context: %d tokens", tokenCount)

	// Stage 7: Final call to target model
	c.log("[curator] Calling %s...", c.config.TargetModel)
	stageStart = time.Now()

	// Build final prompt with curated context
	var finalPrompt string
	if len(decision.MissingCritical) > 0 || decision.Confidence < c.config.ConfidenceThreshold {
		warnings := c.decider.ForceAnswer(needs, organized, gaps)
		finalPrompt = BuildFinalPromptWithWarnings(prompt, curatedContext, warnings)
	} else {
		finalPrompt = BuildFinalPrompt(prompt, curatedContext)
	}

	// Use specified model or default
	targetModel := model
	if targetModel == "" {
		targetModel = c.config.TargetModel
	}

	response, err := c.targetAdapter.Generate(ctx, targetModel, finalPrompt)
	if err != nil {
		return nil, fmt.Errorf("target model error: %w", err)
	}

	result.Stages = append(result.Stages, StageLog{
		Stage:     "generate",
		StartTime: stageStart,
		Duration:  time.Since(stageStart),
		Items:     1,
		Notes:     fmt.Sprintf("Generated response using %s", targetModel),
	})

	result.ProcessingTime = time.Since(startTime)

	// Add curation metadata to the artifact
	response = response.WithMetadata("curator", "true")
	response = response.WithMetadata("needs_count", fmt.Sprintf("%d", len(needs)))
	response = response.WithMetadata("gathered_count", fmt.Sprintf("%d", len(gathered)))
	response = response.WithMetadata("context_tokens", fmt.Sprintf("%d", tokenCount))
	response = response.WithMetadata("processing_time", result.ProcessingTime.String())

	return response, nil
}

// createClarifyingResponse creates an artifact with clarifying questions.
func (c *Curator) createClarifyingResponse(questions []string, model, prompt string) (*artifact.Artifact, error) {
	content := "I need some clarification before I can fully answer your question:\n\n"
	for i, q := range questions {
		content += fmt.Sprintf("%d. %s\n", i+1, q)
	}
	content += "\nPlease provide more details so I can give you a better answer."

	art := artifact.New(content, c.Name(), model, prompt)
	art = art.WithMetadata("type", "clarification")
	art = art.WithMetadata("questions_count", fmt.Sprintf("%d", len(questions)))

	return art, nil
}

// countActiveSources returns the number of available sources.
func (c *Curator) countActiveSources() int {
	return len(c.registry.Available())
}

// log writes to the curator's logger.
func (c *Curator) log(format string, args ...any) {
	if c.config.Debug && c.logger != nil {
		c.logger(format, args...)
	}
}

func defaultLogger(format string, args ...any) {
	log.Printf(format, args...)
}

// GetRegistry returns the source registry for external configuration.
func (c *Curator) GetRegistry() *sources.SourceRegistry {
	return c.registry
}

// GetConfig returns the current configuration.
func (c *Curator) GetConfig() CuratorConfig {
	return c.config
}

// SetDebug enables or disables debug logging.
func (c *Curator) SetDebug(debug bool) {
	c.config.Debug = debug
}

// StoreArtifact stores an artifact in the artifact source if available.
func (c *Curator) StoreArtifact(art *artifact.Artifact) {
	source, ok := c.registry.Get("artifacts")
	if !ok {
		return
	}
	if artSource, ok := source.(*sources.ArtifactSource); ok {
		artSource.Store(art)
	}
}

// StoreConversation stores a conversation turn in the memory source if available.
func (c *Curator) StoreConversation(role, content string) {
	source, ok := c.registry.Get("memory")
	if !ok {
		return
	}
	if memSource, ok := source.(*sources.MemorySource); ok {
		memSource.StoreConversation(role, content)
	}
}
