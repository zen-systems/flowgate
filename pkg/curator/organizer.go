package curator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/zen-systems/flowgate/pkg/adapter"
)

// Organizer ranks and filters gathered information by relevance.
type Organizer struct {
	adapter       adapter.Adapter
	model         string
	contextBudget int // Max tokens to keep
}

// NewOrganizer creates a new Organizer.
func NewOrganizer(a adapter.Adapter, model string, contextBudget int) *Organizer {
	return &Organizer{
		adapter:       a,
		model:         model,
		contextBudget: contextBudget,
	}
}

// Organize filters and ranks gathered information by relevance to the query.
func (o *Organizer) Organize(ctx context.Context, query string, needs []InformationNeed, gathered []GatheredInfo) ([]GatheredInfo, error) {
	if len(gathered) == 0 {
		return gathered, nil
	}

	// For small sets, use simple heuristics
	if len(gathered) <= 5 {
		return o.organizeSimple(gathered, needs)
	}

	// For larger sets, use model to score relevance
	scored, err := o.scoreWithModel(ctx, query, gathered)
	if err != nil {
		// Fall back to simple organization on error
		return o.organizeSimple(gathered, needs)
	}

	// Filter low-relevance items
	filtered := filterByRelevance(scored, 0.2)

	// Sort by relevance
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Relevance > filtered[j].Relevance
	})

	// Trim to context budget
	return o.trimToContextBudget(filtered), nil
}

// organizeSimple uses heuristics to organize without model calls.
func (o *Organizer) organizeSimple(gathered []GatheredInfo, needs []InformationNeed) ([]GatheredInfo, error) {
	// Create need priority map
	needPriority := make(map[string]int)
	for _, need := range needs {
		needPriority[need.ID] = need.Priority
	}

	// Score each item based on confidence and need priority
	for i := range gathered {
		priority := needPriority[gathered[i].NeedID]
		if priority == 0 {
			priority = 5 // default
		}
		// Combine confidence with need priority
		gathered[i].Relevance = gathered[i].Confidence * (float64(priority) / 10.0)
	}

	// Sort by relevance
	sort.Slice(gathered, func(i, j int) bool {
		return gathered[i].Relevance > gathered[j].Relevance
	})

	return o.trimToContextBudget(gathered), nil
}

// scoreWithModel uses a model to score relevance of gathered info.
func (o *Organizer) scoreWithModel(ctx context.Context, query string, gathered []GatheredInfo) ([]GatheredInfo, error) {
	// Build a summary of gathered items for the model
	var itemSummaries []string
	for i, info := range gathered {
		// Truncate long content for the scoring prompt
		content := info.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		itemSummaries = append(itemSummaries, fmt.Sprintf(
			"[%d] Source: %s, Path: %s\nContent preview: %s",
			i, info.Source, info.SourcePath, content,
		))
	}

	prompt := fmt.Sprintf(`Score the relevance of each information item to this query.

Query: %s

Items:
%s

For each item, return a relevance score from 0.0 (not relevant) to 1.0 (highly relevant).
Return ONLY a JSON object mapping item index to score, e.g.: {"0": 0.8, "1": 0.3, "2": 0.95}`,
		query, strings.Join(itemSummaries, "\n\n"))

	result, err := o.adapter.Generate(ctx, o.model, prompt)
	if err != nil {
		return nil, err
	}

	// Parse scores
	scores, err := parseScores(result.Content)
	if err != nil {
		return nil, err
	}

	// Apply scores to gathered info
	for i := range gathered {
		if score, ok := scores[i]; ok {
			gathered[i].Relevance = score
		} else {
			gathered[i].Relevance = gathered[i].Confidence // fallback
		}
	}

	return gathered, nil
}

// parseScores extracts relevance scores from model response.
func parseScores(content string) (map[int]float64, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Try to find JSON object
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var rawScores map[string]float64
	if err := json.Unmarshal([]byte(content), &rawScores); err != nil {
		return nil, fmt.Errorf("failed to parse scores: %w", err)
	}

	// Convert string keys to int
	scores := make(map[int]float64)
	for key, score := range rawScores {
		var idx int
		if _, err := fmt.Sscanf(key, "%d", &idx); err == nil {
			scores[idx] = score
		}
	}

	return scores, nil
}

// filterByRelevance removes items below the threshold.
func filterByRelevance(gathered []GatheredInfo, threshold float64) []GatheredInfo {
	filtered := make([]GatheredInfo, 0, len(gathered))
	for _, info := range gathered {
		if info.Relevance >= threshold {
			filtered = append(filtered, info)
		}
	}
	return filtered
}

// trimToContextBudget removes items to fit within token budget.
func (o *Organizer) trimToContextBudget(gathered []GatheredInfo) []GatheredInfo {
	if o.contextBudget <= 0 {
		return gathered
	}

	var result []GatheredInfo
	totalTokens := 0

	for _, info := range gathered {
		// Rough token estimate: 4 chars per token
		tokens := len(info.Content) / 4
		if totalTokens+tokens > o.contextBudget {
			break
		}
		totalTokens += tokens
		result = append(result, info)
	}

	return result
}

// OrganizeSimple is an alias for direct simple organization.
func (o *Organizer) OrganizeSimple(gathered []GatheredInfo, needs []InformationNeed) ([]GatheredInfo, error) {
	return o.organizeSimple(gathered, needs)
}

// DeduplicateByContent removes duplicate information based on content similarity.
func DeduplicateByContent(gathered []GatheredInfo) []GatheredInfo {
	seen := make(map[string]bool)
	result := make([]GatheredInfo, 0, len(gathered))

	for _, info := range gathered {
		// Create a simple hash of the content
		key := normalizeForDedup(info.Content)
		if !seen[key] {
			seen[key] = true
			result = append(result, info)
		}
	}

	return result
}

// normalizeForDedup creates a normalized version of content for deduplication.
func normalizeForDedup(content string) string {
	// Simple normalization: lowercase, remove extra whitespace, take first 200 chars
	content = strings.ToLower(content)
	content = strings.Join(strings.Fields(content), " ")
	if len(content) > 200 {
		content = content[:200]
	}
	return content
}

// GroupByNeed groups gathered info by which need it satisfies.
func GroupByNeed(gathered []GatheredInfo) map[string][]GatheredInfo {
	groups := make(map[string][]GatheredInfo)
	for _, info := range gathered {
		groups[info.NeedID] = append(groups[info.NeedID], info)
	}
	return groups
}

// TopPerNeed returns the top N items for each need.
func TopPerNeed(gathered []GatheredInfo, n int) []GatheredInfo {
	groups := GroupByNeed(gathered)
	var result []GatheredInfo

	for _, items := range groups {
		// Items should already be sorted by relevance
		count := n
		if count > len(items) {
			count = len(items)
		}
		result = append(result, items[:count]...)
	}

	return result
}
