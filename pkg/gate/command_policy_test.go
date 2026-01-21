package gate

import (
	"path/filepath"
	"testing"
)

func TestCapabilityGoTestTemplates(t *testing.T) {
	templates, ok := TemplatesForCapability("go_test")
	if !ok {
		t.Fatalf("expected go_test capability")
	}

	okMatch, _ := matchTemplates([]string{"go", "test", "./..."}, templates, "/workspace", "")
	if !okMatch {
		t.Fatalf("expected go test ./... to be allowed")
	}

	okMatch, _ = matchTemplates([]string{"go", "test", "-exec", "rm", "-rf", "/"}, templates, "/workspace", "")
	if okMatch {
		t.Fatalf("expected unsafe go test args to be denied")
	}
}

func TestGofmtTemplateWorkspaceConfined(t *testing.T) {
	templates, ok := TemplatesForCapability("gofmt")
	if !ok {
		t.Fatalf("expected gofmt capability")
	}

	workspace := t.TempDir()
	okMatch, reason := matchTemplates([]string{"gofmt", "-w", "foo/bar.go"}, templates, workspace, "")
	if !okMatch {
		t.Fatalf("expected gofmt path to be allowed: %s", reason)
	}
}

func TestGofmtTemplateRejectsBadPaths(t *testing.T) {
	templates, ok := TemplatesForCapability("gofmt")
	if !ok {
		t.Fatalf("expected gofmt capability")
	}

	workspace := t.TempDir()
	okMatch, _ := matchTemplates([]string{"gofmt", "-w", "/etc/passwd"}, templates, workspace, "")
	if okMatch {
		t.Fatalf("expected absolute path to be denied")
	}

	okMatch, _ = matchTemplates([]string{"gofmt", "-w", "../secrets.txt"}, templates, workspace, "")
	if okMatch {
		t.Fatalf("expected traversal path to be denied")
	}
}

func TestWorkdirEscapesWorkspace(t *testing.T) {
	workspace := t.TempDir()
	otherDir := t.TempDir()
	if filepath.Clean(workspace) == filepath.Clean(otherDir) {
		t.Fatalf("expected different temp dirs")
	}

	ok, reason := isWorkspaceConfined(otherDir, workspace, "file.go")
	if ok {
		t.Fatalf("expected workdir escape to be denied")
	}
	if reason == "" {
		t.Fatalf("expected reason for workdir escape")
	}
}
