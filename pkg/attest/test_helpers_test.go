package attest

import (
	"encoding/json"
	"os"
	"testing"
)

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
