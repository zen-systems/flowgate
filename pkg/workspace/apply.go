package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	fileModeDefault = 0644
)

// ApplyResult describes changes made to the workspace.
type ApplyResult struct {
	AppliedFiles    []string `json:"applied_files"`
	DeletedFiles    []string `json:"deleted_files,omitempty"`
	UsedUnifiedDiff bool     `json:"used_unified_diff"`
}

// ApplyOutput applies either a unified diff or file-block output to the workspace.
func ApplyOutput(workspacePath, output string) (*ApplyResult, error) {
	patches, err := ParseUnifiedDiff(output)
	if err == nil {
		return applyPatches(workspacePath, patches)
	}

	files := ParseFileBlocks(output)
	if len(files) == 0 {
		return nil, fmt.Errorf("unable to parse output as unified diff or file blocks: %w", err)
	}

	return applyFileBlocks(workspacePath, files)
}

func applyPatches(workspacePath string, patches []FilePatch) (*ApplyResult, error) {
	if len(patches) == 0 {
		return nil, fmt.Errorf("no patches to apply")
	}

	opPlans := make([]fileOp, 0, len(patches))
	for _, patch := range patches {
		plan, err := buildFileOp(workspacePath, patch)
		if err != nil {
			return nil, err
		}
		opPlans = append(opPlans, plan)
	}

	result := &ApplyResult{UsedUnifiedDiff: true}
	for _, plan := range opPlans {
		if plan.delete {
			if err := os.Remove(plan.path); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			result.DeletedFiles = append(result.DeletedFiles, plan.relative)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(plan.path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(plan.path, []byte(plan.content), plan.mode); err != nil {
			return nil, err
		}
		result.AppliedFiles = append(result.AppliedFiles, plan.relative)
	}

	return result, nil
}

func applyFileBlocks(workspacePath string, files map[string]string) (*ApplyResult, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no file blocks to apply")
	}

	opPlans := make([]fileOp, 0, len(files))
	for rel, content := range files {
		path, err := safeJoin(workspacePath, rel)
		if err != nil {
			return nil, err
		}

		mode := os.FileMode(fileModeDefault)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}

		opPlans = append(opPlans, fileOp{
			path:     path,
			relative: rel,
			content:  content,
			mode:     mode,
		})
	}

	result := &ApplyResult{UsedUnifiedDiff: false}
	for _, plan := range opPlans {
		if err := os.MkdirAll(filepath.Dir(plan.path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(plan.path, []byte(plan.content), plan.mode); err != nil {
			return nil, err
		}
		result.AppliedFiles = append(result.AppliedFiles, plan.relative)
	}

	return result, nil
}

type fileOp struct {
	path     string
	relative string
	content  string
	mode     os.FileMode
	delete   bool
}

func buildFileOp(workspacePath string, patch FilePatch) (fileOp, error) {
	oldPath := normalizeDiffPath(patch.OldPath)
	newPath := normalizeDiffPath(patch.NewPath)

	if newPath == "/dev/null" {
		if oldPath == "/dev/null" {
			return fileOp{}, fmt.Errorf("invalid patch with both paths /dev/null")
		}
		path, err := safeJoin(workspacePath, oldPath)
		if err != nil {
			return fileOp{}, err
		}
		return fileOp{path: path, relative: oldPath, delete: true}, nil
	}

	path, err := safeJoin(workspacePath, newPath)
	if err != nil {
		return fileOp{}, err
	}

	mode := os.FileMode(fileModeDefault)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	var original string
	if oldPath != "/dev/null" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return fileOp{}, err
		}
		if err == nil {
			original = string(data)
		}
	}

	updated, err := applyHunks(original, patch.Hunks)
	if err != nil {
		return fileOp{}, fmt.Errorf("apply patch %s: %w", newPath, err)
	}

	return fileOp{
		path:     path,
		relative: newPath,
		content:  updated,
		mode:     mode,
	}, nil
}

func normalizeDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}

func safeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}
	cleaned := filepath.Clean(rel)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("invalid path: %s", rel)
	}

	joined := filepath.Join(root, cleaned)
	relCheck, err := filepath.Rel(root, joined)
	if err != nil || strings.HasPrefix(relCheck, "..") {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	return joined, nil
}

// ParseFileBlocks extracts files from content marked with file headers.
func ParseFileBlocks(content string) map[string]string {
	files := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentFile string
	var currentContent strings.Builder

	for _, line := range lines {
		if path := extractFilePath(line); path != "" {
			if currentFile != "" {
				files[currentFile] = strings.TrimSuffix(currentContent.String(), "\n")
			}
			currentFile = path
			currentContent.Reset()
			continue
		}

		if currentFile != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	if currentFile != "" {
		files[currentFile] = strings.TrimSuffix(currentContent.String(), "\n")
	}

	return files
}

func extractFilePath(line string) string {
	line = strings.TrimSpace(line)
	prefixes := []string{
		"// file:",
		"// File:",
		"# file:",
		"# File:",
		"/* file:",
		"<!-- file:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			path = strings.TrimSuffix(path, "*/")
			path = strings.TrimSuffix(path, "-->")
			return strings.TrimSpace(path)
		}
	}
	return ""
}
