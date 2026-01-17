package curator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zen-systems/flowgate/pkg/adapter"
)

// Reconciler detects conflicts and gaps in gathered information.
type Reconciler struct {
	adapter           adapter.Adapter
	model             string
	stalenessWindow   time.Duration // How old before info is considered stale
}

// NewReconciler creates a new Reconciler.
func NewReconciler(a adapter.Adapter, model string) *Reconciler {
	return &Reconciler{
		adapter:         a,
		model:           model,
		stalenessWindow: 7 * 24 * time.Hour, // 7 days default
	}
}

// WithStalenessWindow sets how old information must be to be considered stale.
func (r *Reconciler) WithStalenessWindow(d time.Duration) *Reconciler {
	r.stalenessWindow = d
	return r
}

// Reconcile identifies conflicts and gaps in gathered information.
func (r *Reconciler) Reconcile(ctx context.Context, needs []InformationNeed, gathered []GatheredInfo) ([]Conflict, []Gap, error) {
	var conflicts []Conflict
	var gaps []Gap

	// Check for conflicts
	foundConflicts, err := r.findConflicts(ctx, gathered)
	if err != nil {
		// Non-fatal - continue with what we have
		foundConflicts = r.findConflictsSimple(gathered)
	}
	conflicts = foundConflicts

	// Check for gaps
	gaps = r.findGaps(needs, gathered)

	// Mark stale information
	r.markStaleInfo(gathered)

	return conflicts, gaps, nil
}

// findConflicts uses a model to detect contradictory information.
func (r *Reconciler) findConflicts(ctx context.Context, gathered []GatheredInfo) ([]Conflict, error) {
	if len(gathered) < 2 {
		return nil, nil
	}

	// Group by topic (using need ID as proxy)
	groups := GroupByNeed(gathered)

	var allConflicts []Conflict
	conflictID := 0

	for needID, items := range groups {
		if len(items) < 2 {
			continue
		}

		// Build summaries for conflict detection
		var summaries []string
		for i, info := range items {
			content := info.Content
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			summaries = append(summaries, fmt.Sprintf("[%d] %s: %s", i, info.SourcePath, content))
		}

		prompt := fmt.Sprintf(`Analyze these information items for contradictions.

Items (all about the same topic):
%s

Are there any contradictions between these items? If yes, explain what contradicts what.
Return JSON: {"has_conflict": true/false, "topic": "brief topic description", "explanation": "what contradicts if any"}`,
			strings.Join(summaries, "\n\n"))

		result, err := r.adapter.Generate(ctx, r.model, prompt)
		if err != nil {
			continue // Skip this group on error
		}

		// Parse response
		conflict := parseConflictResponse(result.Content, needID, items)
		if conflict != nil {
			conflictID++
			conflict.ID = fmt.Sprintf("conflict_%d", conflictID)
			allConflicts = append(allConflicts, *conflict)
		}
	}

	return allConflicts, nil
}

// parseConflictResponse extracts conflict info from model response.
func parseConflictResponse(content string, needID string, items []GatheredInfo) *Conflict {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var response struct {
		HasConflict bool   `json:"has_conflict"`
		Topic       string `json:"topic"`
		Explanation string `json:"explanation"`
	}

	if err := json.Unmarshal([]byte(content), &response); err != nil {
		return nil
	}

	if !response.HasConflict {
		return nil
	}

	return &Conflict{
		Topic:      response.Topic,
		Claims:     items,
		Resolution: "",
		Resolved:   false,
		Reason:     response.Explanation,
	}
}

// findConflictsSimple detects obvious conflicts without model calls.
func (r *Reconciler) findConflictsSimple(gathered []GatheredInfo) []Conflict {
	// Simple heuristic: look for items with same need but very different content
	groups := GroupByNeed(gathered)
	var conflicts []Conflict
	conflictID := 0

	for _, items := range groups {
		if len(items) < 2 {
			continue
		}

		// Check for timestamp-based conflicts (newer vs older)
		var newest, oldest GatheredInfo
		var hasNewest, hasOldest bool

		for _, item := range items {
			if !hasNewest || item.Timestamp.After(newest.Timestamp) {
				newest = item
				hasNewest = true
			}
			if !hasOldest || item.Timestamp.Before(oldest.Timestamp) {
				oldest = item
				hasOldest = true
			}
		}

		// If there's a significant time gap and different content, flag potential conflict
		if hasNewest && hasOldest {
			timeDiff := newest.Timestamp.Sub(oldest.Timestamp)
			if timeDiff > 24*time.Hour && oldest.Content != newest.Content {
				// Check if content is substantially different
				if !isSimilarContent(oldest.Content, newest.Content) {
					conflictID++
					conflicts = append(conflicts, Conflict{
						ID:         fmt.Sprintf("conflict_%d", conflictID),
						Topic:      oldest.NeedID,
						Claims:     []GatheredInfo{oldest, newest},
						Resolution: "Prefer newer information",
						Resolved:   true,
						Reason:     fmt.Sprintf("Information from different times (%s apart)", formatDuration(timeDiff)),
					})
				}
			}
		}
	}

	return conflicts
}

// isSimilarContent checks if two pieces of content are similar.
func isSimilarContent(a, b string) bool {
	// Simple similarity check: normalize and compare prefix
	aNorm := normalizeForDedup(a)
	bNorm := normalizeForDedup(b)

	if aNorm == bNorm {
		return true
	}

	// Check overlap
	minLen := len(aNorm)
	if len(bNorm) < minLen {
		minLen = len(bNorm)
	}

	if minLen < 50 {
		return aNorm == bNorm
	}

	// Check if significant portion matches
	matches := 0
	for i := 0; i < minLen; i++ {
		if aNorm[i] == bNorm[i] {
			matches++
		}
	}

	return float64(matches)/float64(minLen) > 0.8
}

// findGaps identifies information needs that weren't satisfied.
func (r *Reconciler) findGaps(needs []InformationNeed, gathered []GatheredInfo) []Gap {
	// Track which needs are satisfied
	satisfied := make(map[string]bool)
	satisfiedConfidence := make(map[string]float64)

	for _, info := range gathered {
		if info.Confidence > satisfiedConfidence[info.NeedID] {
			satisfiedConfidence[info.NeedID] = info.Confidence
		}
		if info.Confidence >= 0.5 { // Threshold for "satisfied"
			satisfied[info.NeedID] = true
		}
	}

	var gaps []Gap
	for _, need := range needs {
		if !satisfied[need.ID] {
			gap := Gap{
				NeedID:    need.ID,
				Need:      need.Query,
				Attempted: need.Sources,
				Critical:  need.Required,
			}

			// Determine reason for gap
			if conf, ok := satisfiedConfidence[need.ID]; ok && conf > 0 {
				gap.Reason = fmt.Sprintf("Found information but confidence too low (%.0f%%)", conf*100)
			} else {
				gap.Reason = "No relevant information found in any source"
			}

			// Generate clarifying question
			gap.ClarifyingQ = generateClarifyingQuestion(need)

			gaps = append(gaps, gap)
		}
	}

	return gaps
}

// generateClarifyingQuestion creates a question to ask the user.
func generateClarifyingQuestion(need InformationNeed) string {
	switch need.Type {
	case InfoTypeFact:
		return fmt.Sprintf("Could you tell me %s?", need.Query)
	case InfoTypeCurrentState:
		return fmt.Sprintf("What is the current state of %s?", need.Query)
	case InfoTypeContext:
		return fmt.Sprintf("Can you provide more context about %s?", need.Query)
	case InfoTypeExample:
		return fmt.Sprintf("Could you give me an example of %s?", need.Query)
	case InfoTypeDefinition:
		return fmt.Sprintf("How would you define %s?", need.Query)
	default:
		return fmt.Sprintf("I need more information about: %s", need.Query)
	}
}

// markStaleInfo marks gathered info that may be outdated.
func (r *Reconciler) markStaleInfo(gathered []GatheredInfo) {
	now := time.Now()
	for i := range gathered {
		if now.Sub(gathered[i].Timestamp) > r.stalenessWindow {
			if gathered[i].Metadata == nil {
				gathered[i].Metadata = make(map[string]string)
			}
			gathered[i].Metadata["stale"] = "true"
			gathered[i].Metadata["age"] = formatDuration(now.Sub(gathered[i].Timestamp))
		}
	}
}

// ResolveConflict attempts to resolve a conflict by preferring certain sources.
func ResolveConflict(conflict *Conflict, preferredSources []SourceType) {
	if len(conflict.Claims) == 0 {
		return
	}

	// Build preference order
	sourcePreference := make(map[SourceType]int)
	for i, s := range preferredSources {
		sourcePreference[s] = len(preferredSources) - i // Higher = more preferred
	}

	// Find best claim
	var best *GatheredInfo
	bestScore := -1

	for i := range conflict.Claims {
		claim := &conflict.Claims[i]
		score := sourcePreference[claim.Source]

		// Also consider recency
		if best == nil || claim.Timestamp.After(best.Timestamp) {
			score += 1
		}

		if score > bestScore {
			best = claim
			bestScore = score
		}
	}

	if best != nil {
		conflict.Resolution = fmt.Sprintf("Preferring information from %s (%s)", best.Source, best.SourcePath)
		conflict.Resolved = true
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
