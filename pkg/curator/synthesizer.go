package curator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zen-systems/flowgate/pkg/adapter"
)

// Synthesizer builds the final curated context from gathered information.
type Synthesizer struct {
	adapter       adapter.Adapter
	model         string
	contextBudget int
}

// NewSynthesizer creates a new Synthesizer.
func NewSynthesizer(a adapter.Adapter, model string, contextBudget int) *Synthesizer {
	return &Synthesizer{
		adapter:       a,
		model:         model,
		contextBudget: contextBudget,
	}
}

// Synthesize creates a coherent context from gathered information.
func (s *Synthesizer) Synthesize(ctx context.Context, query string, gathered []GatheredInfo, conflicts []Conflict) (string, int, error) {
	// For small amounts of info, use simple formatting
	if len(gathered) <= 3 {
		context := s.formatSimple(gathered, conflicts)
		return context, estimateTokens(context), nil
	}

	// For larger amounts, use model to synthesize
	synthesized, err := s.synthesizeWithModel(ctx, query, gathered, conflicts)
	if err != nil {
		// Fallback to simple formatting
		context := s.formatSimple(gathered, conflicts)
		return context, estimateTokens(context), nil
	}

	return synthesized, estimateTokens(synthesized), nil
}

// formatSimple creates a straightforward context without model assistance.
func (s *Synthesizer) formatSimple(gathered []GatheredInfo, conflicts []Conflict) string {
	var sections []string

	// Group by source for better organization
	bySource := make(map[SourceType][]GatheredInfo)
	for _, info := range gathered {
		bySource[info.Source] = append(bySource[info.Source], info)
	}

	// Format each source group
	sourceOrder := []SourceType{SourceFilesystem, SourceArtifacts, SourceMemory, SourceWeb}
	for _, source := range sourceOrder {
		items, ok := bySource[source]
		if !ok || len(items) == 0 {
			continue
		}

		// Sort by relevance within source
		sort.Slice(items, func(i, j int) bool {
			return items[i].Relevance > items[j].Relevance
		})

		section := fmt.Sprintf("## From %s:\n\n", formatSourceName(source))
		for _, info := range items {
			section += fmt.Sprintf("### %s\n", info.SourcePath)
			section += info.Content + "\n\n"
		}
		sections = append(sections, section)
	}

	// Add conflict notes if any
	if len(conflicts) > 0 {
		conflictSection := "## Notes on conflicting information:\n\n"
		for _, conflict := range conflicts {
			if conflict.Resolved {
				conflictSection += fmt.Sprintf("- %s: %s\n", conflict.Topic, conflict.Resolution)
			} else {
				conflictSection += fmt.Sprintf("- Unresolved conflict about %s: %s\n", conflict.Topic, conflict.Reason)
			}
		}
		sections = append(sections, conflictSection)
	}

	return strings.Join(sections, "\n")
}

// synthesizeWithModel uses a model to create a coherent summary.
func (s *Synthesizer) synthesizeWithModel(ctx context.Context, query string, gathered []GatheredInfo, conflicts []Conflict) (string, error) {
	// Build input for the model
	var infoSections []string
	for i, info := range gathered {
		section := fmt.Sprintf("[Source %d: %s - %s]\n%s", i+1, info.Source, info.SourcePath, info.Content)
		infoSections = append(infoSections, section)
	}

	var conflictNotes []string
	for _, conflict := range conflicts {
		note := fmt.Sprintf("Conflict about %s: %s", conflict.Topic, conflict.Reason)
		if conflict.Resolved {
			note += fmt.Sprintf(" (Resolved: %s)", conflict.Resolution)
		}
		conflictNotes = append(conflictNotes, note)
	}

	prompt := fmt.Sprintf(`Synthesize this information into a coherent context for answering a query.

Original Query: %s

Gathered Information:
%s

%sCreate a clear, well-organized context summary that:
1. Groups related information logically
2. Highlights the most relevant details for the query
3. Notes any important caveats or conflicts
4. Stays concise while being comprehensive

Format the output as markdown sections. Do not answer the query itself - just organize the context.`,
		query,
		strings.Join(infoSections, "\n\n---\n\n"),
		formatConflictNotes(conflictNotes))

	result, err := s.adapter.Generate(ctx, s.model, prompt)
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

// SynthesizeForPrompt creates a context specifically formatted for use in a prompt.
func (s *Synthesizer) SynthesizeForPrompt(ctx context.Context, query string, gathered []GatheredInfo, conflicts []Conflict) (string, error) {
	context, _, err := s.Synthesize(ctx, query, gathered, conflicts)
	if err != nil {
		return "", err
	}

	// Wrap in clear delimiters
	return fmt.Sprintf("<curated_context>\n%s\n</curated_context>", context), nil
}

// SynthesizeSimple creates a simple context without model calls.
func (s *Synthesizer) SynthesizeSimple(gathered []GatheredInfo, conflicts []Conflict) string {
	return s.formatSimple(gathered, conflicts)
}

// formatSourceName returns a human-readable source name.
func formatSourceName(source SourceType) string {
	switch source {
	case SourceFilesystem:
		return "Local Files"
	case SourceArtifacts:
		return "Previous Outputs"
	case SourceMemory:
		return "Conversation History"
	case SourceWeb:
		return "Web Search"
	default:
		return string(source)
	}
}

func formatConflictNotes(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	return fmt.Sprintf("Conflicts/Caveats:\n- %s\n\n", strings.Join(notes, "\n- "))
}

// estimateTokens provides a rough token count.
func estimateTokens(text string) int {
	// Rough estimate: ~4 characters per token for English text
	return len(text) / 4
}

// TrimToTokenBudget removes content to fit within a token budget.
func TrimToTokenBudget(content string, maxTokens int) string {
	currentTokens := estimateTokens(content)
	if currentTokens <= maxTokens {
		return content
	}

	// Trim from the end, trying to preserve complete sections
	lines := strings.Split(content, "\n")
	var result []string
	tokens := 0

	for _, line := range lines {
		lineTokens := estimateTokens(line) + 1 // +1 for newline
		if tokens+lineTokens > maxTokens {
			break
		}
		tokens += lineTokens
		result = append(result, line)
	}

	if len(result) > 0 {
		return strings.Join(result, "\n") + "\n\n[Context truncated to fit budget]"
	}

	// If even first line is too long, truncate it
	maxChars := maxTokens * 4
	if len(content) > maxChars {
		return content[:maxChars-50] + "\n\n[Context truncated to fit budget]"
	}

	return content
}

// BuildFinalPrompt creates the final prompt with curated context.
func BuildFinalPrompt(query, curatedContext string) string {
	return fmt.Sprintf(`I've gathered relevant context to help answer your question.

%s

Based on the above context, please answer:
%s`, curatedContext, query)
}

// BuildFinalPromptWithWarnings includes warnings about gaps or conflicts.
func BuildFinalPromptWithWarnings(query, curatedContext string, warnings []string) string {
	warningText := ""
	if len(warnings) > 0 {
		warningText = "\n\nNote: " + strings.Join(warnings, "; ") + "\n"
	}

	return fmt.Sprintf(`I've gathered relevant context to help answer your question.
%s
%s

Based on the above context, please answer:
%s`, warningText, curatedContext, query)
}
