package sources

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryEntry represents a stored piece of information in memory.
type MemoryEntry struct {
	ID        string
	Content   string
	Type      string // "conversation", "fact", "preference", etc.
	Timestamp time.Time
	Metadata  map[string]string
}

// MemorySource provides access to conversation history and remembered facts.
type MemorySource struct {
	mu       sync.RWMutex
	entries  []MemoryEntry
	maxItems int
}

// MemoryOption configures a MemorySource.
type MemoryOption func(*MemorySource)

// WithMaxMemoryItems sets the maximum number of entries to store.
func WithMaxMemoryItems(max int) MemoryOption {
	return func(m *MemorySource) {
		m.maxItems = max
	}
}

// NewMemorySource creates a new memory source.
func NewMemorySource(opts ...MemoryOption) *MemorySource {
	m := &MemorySource{
		entries:  make([]MemoryEntry, 0),
		maxItems: 1000,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Name returns the source identifier.
func (m *MemorySource) Name() string {
	return "memory"
}

// Available always returns true for memory source.
func (m *MemorySource) Available() bool {
	return true
}

// Query searches memory for relevant entries.
func (m *MemorySource) Query(ctx context.Context, query string) ([]QueryResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	keywords := extractKeywords(queryLower)

	var results []QueryResult
	for _, entry := range m.entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		contentLower := strings.ToLower(entry.Content)
		relevance := calculateRelevance(contentLower, keywords)

		if relevance > 0.1 {
			results = append(results, QueryResult{
				Content:    entry.Content,
				Path:       "memory:" + entry.ID,
				Confidence: relevance,
				Timestamp:  entry.Timestamp,
				Metadata: map[string]string{
					"type":      entry.Type,
					"memory_id": entry.ID,
				},
			})
		}
	}

	sortByConfidence(results)
	return results, nil
}

// Store adds an entry to memory.
func (m *MemorySource) Store(entry MemoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not set
	if entry.ID == "" {
		entry.ID = generateMemoryID()
	}

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	m.entries = append(m.entries, entry)

	// Trim if over capacity (remove oldest entries)
	if len(m.entries) > m.maxItems {
		m.entries = m.entries[len(m.entries)-m.maxItems:]
	}
}

// StoreConversation stores a conversation turn.
func (m *MemorySource) StoreConversation(role, content string) {
	m.Store(MemoryEntry{
		Content: content,
		Type:    "conversation",
		Metadata: map[string]string{
			"role": role,
		},
	})
}

// StoreFact stores a remembered fact.
func (m *MemorySource) StoreFact(fact string, tags ...string) {
	m.Store(MemoryEntry{
		Content: fact,
		Type:    "fact",
		Metadata: map[string]string{
			"tags": strings.Join(tags, ","),
		},
	})
}

// Clear removes all entries from memory.
func (m *MemorySource) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make([]MemoryEntry, 0)
}

// Count returns the number of entries in memory.
func (m *MemorySource) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// GetRecent returns the n most recent entries.
func (m *MemorySource) GetRecent(n int) []MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n > len(m.entries) {
		n = len(m.entries)
	}

	start := len(m.entries) - n
	result := make([]MemoryEntry, n)
	copy(result, m.entries[start:])
	return result
}

var memoryIDCounter int64
var memoryIDMu sync.Mutex

func generateMemoryID() string {
	memoryIDMu.Lock()
	defer memoryIDMu.Unlock()
	memoryIDCounter++
	return "mem_" + itoa(memoryIDCounter)
}
