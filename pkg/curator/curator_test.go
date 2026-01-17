package curator

import (
	"context"
	"testing"

	"github.com/zen-systems/flowgate/pkg/artifact"
	"github.com/zen-systems/flowgate/pkg/curator/sources"
)

// mockAdapter is a simple mock for testing.
type mockAdapter struct {
	response string
	err      error
}

func (m *mockAdapter) Generate(ctx context.Context, model string, prompt string) (*artifact.Artifact, error) {
	if m.err != nil {
		return nil, m.err
	}
	return artifact.New(m.response, "mock", model, prompt), nil
}

func (m *mockAdapter) Name() string {
	return "mock"
}

func (m *mockAdapter) Models() []string {
	return []string{"mock-model"}
}

func TestAnalyzerSimple(t *testing.T) {
	analyzer := NewAnalyzer(&mockAdapter{}, "mock-model")

	tests := []struct {
		name     string
		query    string
		wantMin  int // minimum expected needs
		wantType []InformationType
	}{
		{
			name:    "code query",
			query:   "Show me the function that handles authentication",
			wantMin: 1,
		},
		{
			name:    "documentation query",
			query:   "How do I use the API?",
			wantMin: 1,
		},
		{
			name:    "example query",
			query:   "Give me an example of usage",
			wantMin: 1,
		},
		{
			name:    "history query",
			query:   "What did we discuss before?",
			wantMin: 1,
		},
		{
			name:    "general query",
			query:   "Hello",
			wantMin: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needs := analyzer.AnalyzeSimple(tt.query)
			if len(needs) < tt.wantMin {
				t.Errorf("AnalyzeSimple() got %d needs, want at least %d", len(needs), tt.wantMin)
			}

			// Verify each need has required fields
			for _, need := range needs {
				if need.ID == "" {
					t.Error("Need has empty ID")
				}
				if need.Query == "" {
					t.Error("Need has empty Query")
				}
				if len(need.Sources) == 0 {
					t.Error("Need has no sources")
				}
			}
		})
	}
}

func TestDecider(t *testing.T) {
	tests := []struct {
		name      string
		needs     []InformationNeed
		gathered  []GatheredInfo
		gaps      []Gap
		wantCan   bool
		threshold float64
	}{
		{
			name:      "no needs - can answer",
			needs:     []InformationNeed{},
			gathered:  []GatheredInfo{},
			gaps:      []Gap{},
			wantCan:   true,
			threshold: 0.7,
		},
		{
			name: "satisfied needs - can answer",
			needs: []InformationNeed{
				{ID: "need_1", Required: true},
			},
			gathered: []GatheredInfo{
				{NeedID: "need_1", Confidence: 0.9},
			},
			gaps:      []Gap{},
			wantCan:   true,
			threshold: 0.7,
		},
		{
			name: "unsatisfied required need - cannot answer",
			needs: []InformationNeed{
				{ID: "need_1", Required: true},
			},
			gathered: []GatheredInfo{
				{NeedID: "need_1", Confidence: 0.3}, // Below threshold
			},
			gaps:      []Gap{{NeedID: "need_1", Critical: true}},
			wantCan:   false,
			threshold: 0.7,
		},
		{
			name: "critical gap - cannot answer",
			needs: []InformationNeed{
				{ID: "need_1", Required: true},
			},
			gathered: []GatheredInfo{},
			gaps: []Gap{
				{NeedID: "need_1", Critical: true, ClarifyingQ: "What is need_1?"},
			},
			wantCan:   false,
			threshold: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decider := NewDecider(tt.threshold, 0.8)
			result := decider.Decide(tt.needs, tt.gathered, tt.gaps)

			if result.CanAnswer != tt.wantCan {
				t.Errorf("Decide() CanAnswer = %v, want %v (reason: %s)", result.CanAnswer, tt.wantCan, result.Reason)
			}
		})
	}
}

func TestReconcilerFindGaps(t *testing.T) {
	reconciler := NewReconciler(&mockAdapter{}, "mock-model")

	needs := []InformationNeed{
		{ID: "need_1", Query: "Find authentication code", Required: true},
		{ID: "need_2", Query: "Find logging examples", Required: false},
	}

	gathered := []GatheredInfo{
		{NeedID: "need_1", Confidence: 0.8}, // Satisfied
		// need_2 not satisfied
	}

	_, gaps, err := reconciler.Reconcile(context.Background(), needs, gathered)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if len(gaps) != 1 {
		t.Errorf("Expected 1 gap, got %d", len(gaps))
	}

	if len(gaps) > 0 && gaps[0].NeedID != "need_2" {
		t.Errorf("Expected gap for need_2, got %s", gaps[0].NeedID)
	}
}

func TestOrganizerDeduplication(t *testing.T) {
	gathered := []GatheredInfo{
		{ID: "1", Content: "Hello world this is a test"},
		{ID: "2", Content: "Hello world this is a test"}, // Duplicate
		{ID: "3", Content: "Something completely different"},
	}

	deduped := DeduplicateByContent(gathered)
	if len(deduped) != 2 {
		t.Errorf("DeduplicateByContent() got %d items, want 2", len(deduped))
	}
}

func TestSynthesizerSimple(t *testing.T) {
	synthesizer := NewSynthesizer(&mockAdapter{}, "mock-model", 50000)

	gathered := []GatheredInfo{
		{
			Source:     SourceFilesystem,
			SourcePath: "test.go",
			Content:    "func Test() {}",
			Relevance:  0.8,
		},
		{
			Source:     SourceMemory,
			SourcePath: "memory:1",
			Content:    "Previous discussion about testing",
			Relevance:  0.6,
		},
	}

	conflicts := []Conflict{
		{
			Topic:      "testing approach",
			Resolution: "Use table-driven tests",
			Resolved:   true,
		},
	}

	context := synthesizer.SynthesizeSimple(gathered, conflicts)

	if context == "" {
		t.Error("SynthesizeSimple() returned empty context")
	}

	// Should contain source labels
	if !containsSubstring(context, "Local Files") && !containsSubstring(context, "filesystem") {
		t.Error("Context should mention file source")
	}
}

func TestMemorySource(t *testing.T) {
	mem := sources.NewMemorySource()

	// Store some entries
	mem.StoreConversation("user", "How do I test?")
	mem.StoreConversation("assistant", "Use table-driven tests")
	mem.StoreFact("User prefers Go", "preferences")

	if mem.Count() != 3 {
		t.Errorf("Memory count = %d, want 3", mem.Count())
	}

	// Query memory
	results, err := mem.Query(context.Background(), "test")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected to find results for 'test' query")
	}

	// Test recent retrieval
	recent := mem.GetRecent(2)
	if len(recent) != 2 {
		t.Errorf("GetRecent(2) returned %d items, want 2", len(recent))
	}

	// Test clear
	mem.Clear()
	if mem.Count() != 0 {
		t.Error("Memory should be empty after Clear()")
	}
}

func TestArtifactSource(t *testing.T) {
	artSource := sources.NewArtifactSource()

	// Store some artifacts
	art1 := artifact.New("Test content about authentication", "test", "model", "How does auth work?")
	art2 := artifact.New("Logging implementation", "test", "model", "Show me logging")

	artSource.Store(art1)
	artSource.Store(art2)

	if artSource.Count() != 2 {
		t.Errorf("Artifact count = %d, want 2", artSource.Count())
	}

	// Query artifacts
	results, err := artSource.Query(context.Background(), "authentication")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected to find results for 'authentication' query")
	}

	// Get by ID
	found := artSource.Get(art1.ID)
	if found == nil {
		t.Error("Get() should return the artifact")
	}
}

func TestFilesystemSource(t *testing.T) {
	// Use current directory as test
	fs := sources.NewFilesystemSource(".", sources.WithMaxFiles(10))

	if !fs.Available() {
		t.Skip("Filesystem source not available")
	}

	// Query for Go files - use a general term that should match
	results, err := fs.Query(context.Background(), "func test")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	// Results should have paths if any were found
	for _, r := range results {
		if r.Path == "" {
			t.Error("Result has empty path")
		}
		if r.Confidence < 0 || r.Confidence > 1 {
			t.Errorf("Invalid confidence: %f", r.Confidence)
		}
	}
	t.Logf("Found %d files matching query", len(results))
}

func TestCuratorIntegration(t *testing.T) {
	// Create a mock adapter that returns a simple analysis
	mockAnalysis := &mockAdapter{
		response: `[{"query": "test info", "type": "context", "required": true, "sources": ["memory"], "priority": 5}]`,
	}

	// Create curator with mock adapters
	mem := sources.NewMemorySource()
	mem.StoreFact("Important test fact")

	curator, err := NewCurator(
		mockAnalysis,
		mockAnalysis,
		WithMemorySource(mem),
		WithConfig(CuratorConfig{
			TargetModel:         "mock-model",
			AnalysisModel:       "mock-model",
			ContextBudget:       10000,
			ConfidenceThreshold: 0.5, // Lower threshold for test
			EnabledSources:      []SourceType{SourceMemory},
			Debug:               false,
		}),
	)
	if err != nil {
		t.Fatalf("NewCurator() error = %v", err)
	}

	if curator.Name() != "curator" {
		t.Errorf("Name() = %s, want curator", curator.Name())
	}

	if len(curator.Models()) == 0 {
		t.Error("Models() should return at least one model")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsSubstring(s[1:], substr)))
}
