package sources

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTavilySource_Available(t *testing.T) {
	// Without API key
	ts := NewWebSource()
	ts.apiKey = ""
	if ts.Available() {
		t.Error("Available() should return false without API key")
	}

	// With API key
	ts.apiKey = "test-key"
	if !ts.Available() {
		t.Error("Available() should return true with API key")
	}
}

func TestTavilySource_Name(t *testing.T) {
	ts := NewWebSource()
	if ts.Name() != "web" {
		t.Errorf("Name() = %s, want 'web'", ts.Name())
	}
}

func TestTavilySource_Query(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("failed to listen for httptest server: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req["include_answer"] != false {
			t.Error("include_answer should be false - we want raw data")
		}

		if req["search_depth"] != "advanced" {
			t.Error("search_depth should be 'advanced' for full content")
		}

		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Test Result",
					"url":     "https://example.com/test",
					"content": "This is the raw content from the page",
					"score":   0.95,
				},
				{
					"title":   "Another Result",
					"url":     "https://example.com/other",
					"content": "More raw content here",
					"score":   0.82,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	ts := &TavilySource{
		apiKey:     "test-key",
		httpClient: server.Client(),
		maxResults: 5,
	}

	if !ts.Available() {
		t.Error("Source should be available with API key")
	}

	tsNoKey := NewWebSource()
	tsNoKey.apiKey = ""
	_, err = tsNoKey.Query(context.Background(), "test query")
	if err == nil {
		t.Error("Query should fail without API key")
	}
}

func TestTavilySource_Options(t *testing.T) {
	ts := NewWebSource(
		WithTavilyAPIKey("custom-key"),
		WithMaxResults(10),
	)

	if ts.apiKey != "custom-key" {
		t.Errorf("apiKey = %s, want 'custom-key'", ts.apiKey)
	}

	if ts.maxResults != 10 {
		t.Errorf("maxResults = %d, want 10", ts.maxResults)
	}
}
