package gate

import (
	"testing"
)

func TestParseMultiFileContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "empty content",
			content:  "",
			expected: map[string]string{},
		},
		{
			name:     "single file no marker",
			content:  "package main\n\nfunc main() {}\n",
			expected: map[string]string{},
		},
		{
			name: "single file with marker",
			content: `// file: main.go
package main

func main() {}`,
			expected: map[string]string{
				"main.go": "package main\n\nfunc main() {}",
			},
		},
		{
			name: "multiple files",
			content: `// file: cmd/main.go
package main

import "fmt"

func main() {
	fmt.Println("hello")
}
// file: pkg/util.go
package pkg

func Helper() string {
	return "help"
}`,
			expected: map[string]string{
				"cmd/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}",
				"pkg/util.go": "package pkg\n\nfunc Helper() string {\n\treturn \"help\"\n}",
			},
		},
		{
			name: "python files",
			content: `# file: app.py
def main():
    print("hello")
# file: utils.py
def helper():
    return "help"`,
			expected: map[string]string{
				"app.py":   "def main():\n    print(\"hello\")",
				"utils.py": "def helper():\n    return \"help\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMultiFileContent(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("parseMultiFileContent() returned %d files, want %d", len(result), len(tt.expected))
				return
			}
			for path, content := range tt.expected {
				if got, ok := result[path]; !ok {
					t.Errorf("parseMultiFileContent() missing file %s", path)
				} else if got != content {
					t.Errorf("parseMultiFileContent()[%s] = %q, want %q", path, got, content)
				}
			}
		})
	}
}

func TestExtractFilePath(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"// file: main.go", "main.go"},
		{"// File: pkg/util.go", "pkg/util.go"},
		{"# file: script.py", "script.py"},
		{"# File: config.yaml", "config.yaml"},
		{"/* file: style.css */", "style.css"},
		{"<!-- file: index.html -->", "index.html"},
		{"  // file: indented.go  ", "indented.go"},
		{"not a file marker", ""},
		{"//file:nospace.go", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := extractFilePath(tt.line)
			if result != tt.expected {
				t.Errorf("extractFilePath(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

func TestGenerateRepairHint(t *testing.T) {
	tests := []struct {
		name     string
		issue    hollowcheckIssue
		contains string
	}{
		{
			name: "forbidden_pattern TODO",
			issue: hollowcheckIssue{
				Rule:    "forbidden_pattern",
				File:    "main.go",
				Line:    47,
				Message: "forbidden pattern \"TODO\" found: Work-in-progress marker",
			},
			contains: "Remove TODO comment at main.go:47",
		},
		{
			name: "forbidden_pattern FIXME",
			issue: hollowcheckIssue{
				Rule:    "forbidden_pattern",
				File:    "util.go",
				Line:    12,
				Message: "forbidden pattern \"FIXME\" found",
			},
			contains: "Address FIXME comment at util.go:12",
		},
		{
			name: "forbidden_pattern panic not implemented",
			issue: hollowcheckIssue{
				Rule:    "forbidden_pattern",
				File:    "service.go",
				Line:    100,
				Message: "forbidden pattern \"panic(\\\"not implemented\\\")\" found: Go stub pattern",
			},
			contains: "Replace panic(\"not implemented\") with real implementation at service.go:100",
		},
		{
			name: "stub/low_complexity",
			issue: hollowcheckIssue{
				Rule:    "low_complexity",
				File:    "handler.go",
				Line:    55,
				Message: "Function has cyclomatic complexity of 1",
			},
			contains: "Implement stub function at handler.go:55",
		},
		{
			name: "mock_data",
			issue: hollowcheckIssue{
				Rule:    "mock_data",
				File:    "config.go",
				Line:    10,
				Message: "Mock data pattern found: example.com",
			},
			contains: "Replace placeholder/mock data at config.go:10",
		},
		{
			name: "missing_file",
			issue: hollowcheckIssue{
				Rule:    "missing_file",
				File:    "",
				Line:    0,
				Message: "Required file not found: README.md",
			},
			contains: "Create required file",
		},
		{
			name: "missing_symbol",
			issue: hollowcheckIssue{
				Rule:    "missing_symbol",
				File:    "api.go",
				Line:    0,
				Message: "Required function not found: HandleRequest",
			},
			contains: "Implement required symbol",
		},
		{
			name: "error ignored",
			issue: hollowcheckIssue{
				Rule:    "error-handling",
				File:    "db.go",
				Line:    30,
				Message: "Error return value ignored",
			},
			contains: "Handle error properly at db.go:30",
		},
		{
			name: "empty block",
			issue: hollowcheckIssue{
				Rule:    "no-empty-function",
				File:    "handler.go",
				Line:    55,
				Message: "Empty function body",
			},
			contains: "Add implementation to empty block at handler.go:55",
		},
		{
			name: "generic rule",
			issue: hollowcheckIssue{
				Rule:    "some-other-rule",
				File:    "code.go",
				Line:    10,
				Message: "Some violation",
			},
			contains: "Fix some-other-rule violation at code.go:10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := generateRepairHint(tt.issue)
			if hint == "" {
				t.Error("generateRepairHint() returned empty string")
				return
			}
			if !containsString(hint, tt.contains) {
				t.Errorf("generateRepairHint() = %q, want to contain %q", hint, tt.contains)
			}
		})
	}
}

func TestToGateResult(t *testing.T) {
	g := NewHollowCheckGate("", "")

	t.Run("passing result", func(t *testing.T) {
		output := &hollowcheckOutput{
			Passed:     true,
			Score:      0, // hollowcheck score is penalty points, 0 = perfect
			Violations: nil,
		}
		result := g.toGateResult(output)
		if !result.Passed {
			t.Error("expected Passed=true")
		}
		if result.Score != 0 { // lower is better, 0 = perfect
			t.Errorf("Score = %d, want 0", result.Score)
		}
	})

	t.Run("passing with some violations", func(t *testing.T) {
		output := &hollowcheckOutput{
			Passed: true,  // still passed (under threshold)
			Score:  20,    // 20 penalty points (hollowness)
			Violations: []hollowcheckIssue{
				{
					Rule:     "forbidden_pattern",
					Severity: "error",
					Message:  "forbidden pattern \"TODO\" found",
					File:     "main.go",
					Line:     10,
				},
			},
		}
		result := g.toGateResult(output)
		if !result.Passed {
			t.Error("expected Passed=true")
		}
		if result.Score != 20 { // lower is better, 20% hollowness
			t.Errorf("Score = %d, want 20", result.Score)
		}
	})

	t.Run("failing result with violations", func(t *testing.T) {
		output := &hollowcheckOutput{
			Passed: false,
			Score:  60, // 60 penalty points (hollowness)
			Violations: []hollowcheckIssue{
				{
					Rule:     "forbidden_pattern",
					Severity: "error",
					Message:  "forbidden pattern \"TODO\" found",
					File:     "main.go",
					Line:     10,
				},
				{
					Rule:     "low_complexity",
					Severity: "error",
					Message:  "Stub detected",
					File:     "util.go",
					Line:     20,
				},
			},
		}
		result := g.toGateResult(output)
		if result.Passed {
			t.Error("expected Passed=false")
		}
		if result.Score != 60 { // lower is better, 60% hollowness = bad
			t.Errorf("Score = %d, want 60", result.Score)
		}
		if len(result.Violations) != 2 {
			t.Errorf("got %d violations, want 2", len(result.Violations))
		}
		if len(result.RepairHints) != 2 {
			t.Errorf("got %d repair hints, want 2", len(result.RepairHints))
		}

		// Check location formatting
		if result.Violations[0].Location != "main.go:10" {
			t.Errorf("Location = %q, want main.go:10", result.Violations[0].Location)
		}
		if result.Violations[1].Location != "util.go:20" {
			t.Errorf("Location = %q, want util.go:20", result.Violations[1].Location)
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
