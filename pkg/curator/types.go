// Package curator provides a meta-adapter that self-curates context before responding.
// It orchestrates multiple model calls to build optimal context through analysis,
// gathering, organization, reconciliation, decision-making, and synthesis stages.
package curator

import (
	"time"
)

// InformationType categorizes what kind of information is needed.
type InformationType string

const (
	InfoTypeFact         InformationType = "fact"
	InfoTypeContext      InformationType = "context"
	InfoTypeExample      InformationType = "example"
	InfoTypeDefinition   InformationType = "definition"
	InfoTypeCurrentState InformationType = "current_state"
)

// SourceType identifies where information can be gathered from.
type SourceType string

const (
	SourceFilesystem SourceType = "filesystem"
	SourceWeb        SourceType = "web"
	SourceMemory     SourceType = "memory"
	SourceArtifacts  SourceType = "artifacts"
)

// InformationNeed represents a specific piece of information needed to answer a query.
type InformationNeed struct {
	ID       string          // Unique identifier for this need
	Query    string          // What we need to know
	Type     InformationType // Category of information
	Required bool            // Must have vs nice to have
	Sources  []SourceType    // Suggested sources to check
	Priority int             // Higher = more important (1-10)
}

// GatheredInfo represents information collected from a source.
type GatheredInfo struct {
	ID         string            // Unique identifier
	NeedID     string            // Which InformationNeed this satisfies
	Content    string            // The actual information
	Source     SourceType        // Where it came from
	SourcePath string            // Specific path/URL/reference
	Confidence float64           // 0-1 how confident we are this answers the need
	Timestamp  time.Time         // When this info was gathered/created
	Metadata   map[string]string // Additional context
	Relevance  float64           // 0-1 how relevant to the original query
}

// Conflict represents contradictory information about the same topic.
type Conflict struct {
	ID         string         // Unique identifier
	Topic      string         // What the conflict is about
	Claims     []GatheredInfo // Contradictory information
	Resolution string         // How curator resolved it (or empty if unresolved)
	Resolved   bool           // Whether conflict was resolved
	Reason     string         // Explanation of resolution decision
}

// Gap represents missing information that couldn't be found.
type Gap struct {
	NeedID      string       // Which InformationNeed wasn't satisfied
	Need        string       // Description of what's missing
	Attempted   []SourceType // Sources we tried
	Reason      string       // Why we couldn't find it
	ClarifyingQ string       // Question to ask user if needed
	Critical    bool         // If true, blocks answering
}

// CuratedContext is the final result of the curation process.
type CuratedContext struct {
	Query          string            // Original user query
	Needs          []InformationNeed // Identified information needs
	Gathered       []GatheredInfo    // All gathered information
	Organized      []GatheredInfo    // Filtered and ranked information
	Conflicts      []Conflict        // Detected conflicts
	Gaps           []Gap             // Information gaps
	CanAnswer      bool              // Whether we have enough to answer
	ClarifyingQs   []string          // Questions for user if CanAnswer is false
	FinalContext   string            // Synthesized context to pass to model
	TokenEstimate  int               // Estimated tokens in final context
	ProcessingTime time.Duration     // How long curation took
	Stages         []StageLog        // Log of each processing stage
}

// StageLog records what happened in each curation stage.
type StageLog struct {
	Stage     string        // Stage name
	StartTime time.Time     // When stage started
	Duration  time.Duration // How long it took
	Items     int           // Number of items processed/produced
	Notes     string        // Additional information
}

// CuratorConfig holds configuration for the Curator.
type CuratorConfig struct {
	TargetAdapter       string       // Adapter to use for final generation
	TargetModel         string       // Model to use for final generation
	AnalysisModel       string       // Fast model for analysis stages
	ContextBudget       int          // Max tokens for curated context
	ConfidenceThreshold float64      // Min confidence to proceed without clarification
	EnabledSources      []SourceType // Which sources to use
	MaxGatherParallel   int          // Max parallel gather operations
	MaxGatherPerSource  int          // Max items to gather per source
	Debug               bool         // Enable verbose logging
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() CuratorConfig {
	return CuratorConfig{
		TargetAdapter:       "anthropic",
		TargetModel:         "claude-sonnet-4-20250514",
		AnalysisModel:       "claude-sonnet-4-20250514",
		ContextBudget:       50000,
		ConfidenceThreshold: 0.7,
		EnabledSources:      []SourceType{SourceFilesystem, SourceArtifacts},
		MaxGatherParallel:   10,
		MaxGatherPerSource:  20,
		Debug:               false,
	}
}
