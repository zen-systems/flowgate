package sources

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemSource searches local files for information.
type FilesystemSource struct {
	basePath   string
	extensions []string // File extensions to search (e.g., ".go", ".md")
	maxFiles   int      // Maximum files to search
	maxSize    int64    // Maximum file size to read (bytes)
}

// FilesystemOption configures a FilesystemSource.
type FilesystemOption func(*FilesystemSource)

// WithExtensions sets which file extensions to search.
func WithExtensions(exts []string) FilesystemOption {
	return func(f *FilesystemSource) {
		f.extensions = exts
	}
}

// WithMaxFiles sets the maximum number of files to search.
func WithMaxFiles(max int) FilesystemOption {
	return func(f *FilesystemSource) {
		f.maxFiles = max
	}
}

// WithMaxSize sets the maximum file size to read.
func WithMaxSize(max int64) FilesystemOption {
	return func(f *FilesystemSource) {
		f.maxSize = max
	}
}

// NewFilesystemSource creates a new filesystem source.
func NewFilesystemSource(basePath string, opts ...FilesystemOption) *FilesystemSource {
	f := &FilesystemSource{
		basePath:   basePath,
		extensions: []string{".go", ".md", ".txt", ".yaml", ".yml", ".json"},
		maxFiles:   100,
		maxSize:    1024 * 1024, // 1MB
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Name returns the source identifier.
func (f *FilesystemSource) Name() string {
	return "filesystem"
}

// Available returns true if the base path exists.
func (f *FilesystemSource) Available() bool {
	info, err := os.Stat(f.basePath)
	return err == nil && info.IsDir()
}

// Query searches files for content matching the query.
func (f *FilesystemSource) Query(ctx context.Context, query string) ([]QueryResult, error) {
	var results []QueryResult
	queryLower := strings.ToLower(query)
	keywords := extractKeywords(queryLower)

	fileCount := 0
	err := filepath.WalkDir(f.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if d.IsDir() {
			// Skip hidden and common non-code directories
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file count limit
		if fileCount >= f.maxFiles {
			return filepath.SkipAll
		}

		// Check extension
		ext := filepath.Ext(path)
		if !f.hasExtension(ext) {
			return nil
		}

		// Check file size
		info, err := d.Info()
		if err != nil || info.Size() > f.maxSize {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileCount++

		// Check if file is relevant to query
		contentLower := strings.ToLower(string(content))
		relevance := calculateRelevance(contentLower, keywords)

		if relevance > 0.1 { // Only include somewhat relevant files
			relPath, _ := filepath.Rel(f.basePath, path)
			results = append(results, QueryResult{
				Content:    string(content),
				Path:       relPath,
				Confidence: relevance,
				Timestamp:  info.ModTime(),
				Metadata: map[string]string{
					"type":      "file",
					"extension": ext,
					"size":      formatSize(info.Size()),
				},
			})
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, err
	}

	// Sort by relevance
	sortByConfidence(results)

	return results, nil
}

func (f *FilesystemSource) hasExtension(ext string) bool {
	for _, e := range f.extensions {
		if e == ext {
			return true
		}
	}
	return false
}

// extractKeywords splits a query into searchable keywords.
func extractKeywords(query string) []string {
	// Simple keyword extraction - split on spaces and remove common words
	words := strings.Fields(query)
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"what": true, "how": true, "where": true, "when": true, "why": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"and": true, "or": true, "but": true, "with": true,
	}

	var keywords []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) > 2 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// calculateRelevance scores how relevant content is to keywords.
func calculateRelevance(content string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}

	matches := 0
	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			matches++
		}
	}

	return float64(matches) / float64(len(keywords))
}

// sortByConfidence sorts results by confidence descending.
func sortByConfidence(results []QueryResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Confidence > results[i].Confidence {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(
				string(rune(size))+"B",
				string(rune(0)), "", -1,
			), "0"), ".")
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp < len(units) {
		return strings.TrimRight(strings.TrimRight(
			formatFloat(float64(size)/float64(div), 1)+units[exp],
			"0"), ".")
	}
	return formatFloat(float64(size)/float64(div), 1) + "PB"
}

func formatFloat(f float64, precision int) string {
	format := "%." + string(rune('0'+precision)) + "f"
	return strings.TrimRight(strings.TrimRight(
		sprintf(format, f), "0"), ".")
}

func sprintf(format string, args ...interface{}) string {
	// Simple implementation for formatting
	if len(args) == 0 {
		return format
	}
	switch v := args[0].(type) {
	case float64:
		intPart := int64(v)
		fracPart := int64((v - float64(intPart)) * 10)
		if fracPart == 0 {
			return itoa(intPart)
		}
		return itoa(intPart) + "." + itoa(fracPart)
	default:
		return format
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
