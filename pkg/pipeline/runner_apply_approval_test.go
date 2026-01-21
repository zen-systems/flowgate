package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/zen-systems/flowgate/pkg/adapter"
)

func TestApplyForRealRequiresApproval(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	realWorkspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(realWorkspace, "hello.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("write real workspace: %v", err)
	}

	content := "// file: hello.txt\nmodified\n"
	fakeAdapter := &fileBlockAdapter{content: content}

	p := &Pipeline{
		Name: "apply-real",
		Stages: []*Stage{
			{
				Name:   "stage",
				Prompt: "apply",
				Model:  "fileblock-1",
				Apply:  true,
			},
		},
		Adapters: map[string]adapter.Adapter{"fileblock": fakeAdapter},
	}

	_, err := Run(context.Background(), p, RunOptions{
		Input:         "input",
		WorkspacePath: realWorkspace,
		EvidenceDir:   t.TempDir(),
		ApplyForReal:  true,
	})
	if err == nil {
		t.Fatalf("expected approval error")
	}

	data, err := os.ReadFile(filepath.Join(realWorkspace, "hello.txt"))
	if err != nil {
		t.Fatalf("read real workspace: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("real workspace modified without approval")
	}
}

func TestApplyForRealWithApproval(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	realWorkspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(realWorkspace, "hello.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("write real workspace: %v", err)
	}

	content := "// file: hello.txt\nmodified\n"
	fakeAdapter := &fileBlockAdapter{content: content}

	p := &Pipeline{
		Name: "apply-real",
		Stages: []*Stage{
			{
				Name:   "stage",
				Prompt: "apply",
				Model:  "fileblock-1",
				Apply:  true,
			},
		},
		Adapters: map[string]adapter.Adapter{"fileblock": fakeAdapter},
	}

	_, err := Run(context.Background(), p, RunOptions{
		Input:         "input",
		WorkspacePath: realWorkspace,
		EvidenceDir:   t.TempDir(),
		ApplyForReal:  true,
		ApplyApproved: true,
	})
	if err != nil {
		t.Fatalf("expected apply to succeed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(realWorkspace, "hello.txt"))
	if err != nil {
		t.Fatalf("read real workspace: %v", err)
	}
	if string(data) != "modified\n" {
		t.Fatalf("real workspace not modified with approval")
	}
}
