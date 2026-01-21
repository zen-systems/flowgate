package workspace

import (
	"fmt"
	"strconv"
	"strings"
)

// FilePatch represents a unified diff for a single file.
type FilePatch struct {
	OldPath string
	NewPath string
	Hunks   []Hunk
}

// Hunk represents a unified diff hunk.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []string
}

// ParseUnifiedDiff parses a unified diff into file patches.
func ParseUnifiedDiff(input string) ([]FilePatch, error) {
	lines := strings.Split(input, "\n")
	var patches []FilePatch

	for i := 0; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, "--- ") {
			i++
			continue
		}

		oldPath := parseDiffPath(line)
		i++
		if i >= len(lines) || !strings.HasPrefix(lines[i], "+++ ") {
			return nil, fmt.Errorf("expected +++ after --- for %s", oldPath)
		}
		newPath := parseDiffPath(lines[i])
		i++

		patch := FilePatch{OldPath: oldPath, NewPath: newPath}
		for i < len(lines) && strings.HasPrefix(lines[i], "@@") {
			hunk, next, err := parseHunk(lines, i)
			if err != nil {
				return nil, err
			}
			patch.Hunks = append(patch.Hunks, hunk)
			i = next
		}

		patches = append(patches, patch)
	}

	if len(patches) == 0 {
		return nil, fmt.Errorf("no unified diff content found")
	}
	return patches, nil
}

func parseDiffPath(line string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "---"), "+++"))
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func parseHunk(lines []string, start int) (Hunk, int, error) {
	line := lines[start]
	oldStart, oldLines, newStart, newLines, err := parseHunkHeader(line)
	if err != nil {
		return Hunk{}, 0, err
	}

	hunk := Hunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}

	i := start + 1
	for i < len(lines) {
		if strings.HasPrefix(lines[i], "@@") || strings.HasPrefix(lines[i], "--- ") {
			break
		}
		if strings.HasPrefix(lines[i], "\\") {
			i++
			continue
		}
		hunk.Lines = append(hunk.Lines, lines[i])
		i++
	}

	return hunk, i, nil
}

func parseHunkHeader(line string) (int, int, int, int, error) {
	if !strings.HasPrefix(line, "@@") {
		return 0, 0, 0, 0, fmt.Errorf("invalid hunk header: %s", line)
	}

	trimmed := strings.TrimSpace(strings.TrimPrefix(line, "@@"))
	trimmed = strings.TrimSuffix(trimmed, "@@")
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return 0, 0, 0, 0, fmt.Errorf("invalid hunk header: %s", line)
	}

	oldStart, oldLines, err := parseHunkRange(fields[0])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	newStart, newLines, err := parseHunkRange(fields[1])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return oldStart, oldLines, newStart, newLines, nil
}

func parseHunkRange(value string) (int, int, error) {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return 0, 0, fmt.Errorf("empty hunk range")
	}
	prefix := value[0]
	if prefix != '-' && prefix != '+' {
		return 0, 0, fmt.Errorf("invalid hunk range: %s", value)
	}

	body := value[1:]
	parts := strings.SplitN(body, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hunk start: %s", value)
	}

	lines := 1
	if len(parts) == 2 {
		lines, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid hunk length: %s", value)
		}
	}

	return start, lines, nil
}

func applyHunks(original string, hunks []Hunk) (string, error) {
	oldLines := splitLines(original)
	var newLines []string

	index := 0
	for _, hunk := range hunks {
		if hunk.OldStart < 0 {
			return "", fmt.Errorf("invalid hunk start")
		}

		targetIndex := hunk.OldStart - 1
		if targetIndex < 0 {
			targetIndex = 0
		}

		if targetIndex > len(oldLines) {
			return "", fmt.Errorf("hunk starts beyond file length")
		}

		newLines = append(newLines, oldLines[index:targetIndex]...)
		index = targetIndex

		for _, line := range hunk.Lines {
			if line == "" {
				newLines = append(newLines, "")
				continue
			}

			switch line[0] {
			case ' ': // context
				text := line[1:]
				if index >= len(oldLines) || oldLines[index] != text {
					return "", fmt.Errorf("context mismatch: %s", text)
				}
				newLines = append(newLines, text)
				index++
			case '-':
				text := line[1:]
				if index >= len(oldLines) || oldLines[index] != text {
					return "", fmt.Errorf("delete mismatch: %s", text)
				}
				index++
			case '+':
				newLines = append(newLines, line[1:])
			default:
				return "", fmt.Errorf("invalid hunk line: %s", line)
			}
		}
	}

	newLines = append(newLines, oldLines[index:]...)
	return strings.Join(newLines, "\n"), nil
}

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	if strings.HasSuffix(content, "\n") {
		content = strings.TrimSuffix(content, "\n")
	}
	return strings.Split(content, "\n")
}
