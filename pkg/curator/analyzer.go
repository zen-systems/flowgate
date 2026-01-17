package curator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zen-systems/flowgate/pkg/adapter"
)

// Analyzer breaks down a query into information needs.
type Analyzer struct {
	adapter adapter.Adapter
	model   string
}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer(a adapter.Adapter, model string) *Analyzer {
	return &Analyzer{
		adapter: a,
		model:   model,
	}
}

// Analyze decomposes a query into specific information needs.
func (a *Analyzer) Analyze(ctx context.Context, query string) ([]InformationNeed, error) {
	prompt := fmt.Sprintf(`Analyze this query and identify what information is needed to answer it well.

Query: %s

For each information need, determine:
- query: what specifically we need to know (be specific and actionable)
- type: one of "fact", "context", "example", "definition", "current_state"
- required: true if essential to answer, false if helpful but optional
- sources: array of suggested places to look - "filesystem" (local code/docs), "memory" (conversation history), "artifacts" (previous AI outputs)
- priority: 1-10 where 10 is most important

Return ONLY a JSON array, no markdown or explanation. Example format:
[
  {
    "query": "what authentication mechanism is used",
    "type": "current_state",
    "required": true,
    "sources": ["filesystem"],
    "priority": 9
  }
]`, query)

	result, err := a.adapter.Generate(ctx, a.model, prompt)
	if err != nil {
		return nil, fmt.Errorf("analyzer model call failed: %w", err)
	}

	return parseInformationNeeds(result.Content)
}

// parseInformationNeeds extracts InformationNeed objects from model response.
func parseInformationNeeds(content string) ([]InformationNeed, error) {
	// Clean up response - remove markdown code blocks if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Parse JSON
	var rawNeeds []struct {
		Query    string   `json:"query"`
		Type     string   `json:"type"`
		Required bool     `json:"required"`
		Sources  []string `json:"sources"`
		Priority int      `json:"priority"`
	}

	if err := json.Unmarshal([]byte(content), &rawNeeds); err != nil {
		// Try to extract JSON from the response
		start := strings.Index(content, "[")
		end := strings.LastIndex(content, "]")
		if start >= 0 && end > start {
			content = content[start : end+1]
			if err := json.Unmarshal([]byte(content), &rawNeeds); err != nil {
				return nil, fmt.Errorf("failed to parse needs JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse needs JSON: %w", err)
		}
	}

	// Convert to typed InformationNeed
	needs := make([]InformationNeed, 0, len(rawNeeds))
	for i, raw := range rawNeeds {
		need := InformationNeed{
			ID:       fmt.Sprintf("need_%d", i+1),
			Query:    raw.Query,
			Type:     parseInfoType(raw.Type),
			Required: raw.Required,
			Sources:  parseSources(raw.Sources),
			Priority: raw.Priority,
		}

		// Ensure priority is in valid range
		if need.Priority < 1 {
			need.Priority = 1
		} else if need.Priority > 10 {
			need.Priority = 10
		}

		// Default sources if none specified
		if len(need.Sources) == 0 {
			need.Sources = []SourceType{SourceFilesystem}
		}

		needs = append(needs, need)
	}

	return needs, nil
}

func parseInfoType(t string) InformationType {
	switch strings.ToLower(t) {
	case "fact":
		return InfoTypeFact
	case "context":
		return InfoTypeContext
	case "example":
		return InfoTypeExample
	case "definition":
		return InfoTypeDefinition
	case "current_state":
		return InfoTypeCurrentState
	default:
		return InfoTypeContext
	}
}

func parseSources(sources []string) []SourceType {
	result := make([]SourceType, 0, len(sources))
	for _, s := range sources {
		switch strings.ToLower(s) {
		case "filesystem":
			result = append(result, SourceFilesystem)
		case "web":
			result = append(result, SourceWeb)
		case "memory":
			result = append(result, SourceMemory)
		case "artifacts":
			result = append(result, SourceArtifacts)
		}
	}
	return result
}

// AnalyzeSimple provides a simpler analysis without model calls.
// Useful for basic queries or when model calls should be avoided.
func (a *Analyzer) AnalyzeSimple(query string) []InformationNeed {
	needs := make([]InformationNeed, 0)

	queryLower := strings.ToLower(query)

	// Detect query patterns and create appropriate needs

	// Code-related queries
	if containsAny(queryLower, []string{"function", "method", "class", "type", "struct", "interface"}) {
		needs = append(needs, InformationNeed{
			ID:       "need_code",
			Query:    "Find relevant code definitions",
			Type:     InfoTypeCurrentState,
			Required: true,
			Sources:  []SourceType{SourceFilesystem},
			Priority: 9,
		})
	}

	// Documentation queries
	if containsAny(queryLower, []string{"how to", "documentation", "readme", "explain", "what is"}) {
		needs = append(needs, InformationNeed{
			ID:       "need_docs",
			Query:    "Find relevant documentation",
			Type:     InfoTypeContext,
			Required: true,
			Sources:  []SourceType{SourceFilesystem},
			Priority: 8,
		})
	}

	// Example queries
	if containsAny(queryLower, []string{"example", "sample", "usage", "how do i"}) {
		needs = append(needs, InformationNeed{
			ID:       "need_examples",
			Query:    "Find usage examples",
			Type:     InfoTypeExample,
			Required: false,
			Sources:  []SourceType{SourceFilesystem, SourceArtifacts},
			Priority: 6,
		})
	}

	// History queries
	if containsAny(queryLower, []string{"previous", "before", "earlier", "last time", "we discussed"}) {
		needs = append(needs, InformationNeed{
			ID:       "need_history",
			Query:    "Find relevant conversation history",
			Type:     InfoTypeContext,
			Required: true,
			Sources:  []SourceType{SourceMemory, SourceArtifacts},
			Priority: 7,
		})
	}

	// If no specific patterns detected, create a general need
	if len(needs) == 0 {
		needs = append(needs, InformationNeed{
			ID:       "need_general",
			Query:    query,
			Type:     InfoTypeContext,
			Required: true,
			Sources:  []SourceType{SourceFilesystem, SourceMemory},
			Priority: 5,
		})
	}

	return needs
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
