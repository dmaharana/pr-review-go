package diff

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"pr-review-go/internal/domain"
)

// ParseUnifiedDiff parses a raw unified git diff string into structured DiffFiles.
func ParseUnifiedDiff(rawDiff string) ([]domain.DiffFile, error) {
	if strings.TrimSpace(rawDiff) == "" {
		return nil, nil
	}

	reader := strings.NewReader(rawDiff)
	files, _, err := gitdiff.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parsing git diff: %w", err)
	}

	result := make([]domain.DiffFile, 0, len(files))
	for _, f := range files {
		var patchBuilder strings.Builder
		for _, h := range f.TextFragments {
			patchBuilder.WriteString(h.String())
		}

		oldPath := f.OldName
		if oldPath == "" {
			oldPath = "/dev/null"
		}
		newPath := f.NewName
		if newPath == "" {
			newPath = "/dev/null"
		}

		result = append(result, domain.DiffFile{
			OldPath:   oldPath,
			NewPath:   newPath,
			Patch:     patchBuilder.String(),
			IsNew:     f.IsNew,
			IsDeleted: f.IsDelete,
			IsBinary:  f.IsBinary,
		})
	}
	return result, nil
}

// FilterFiles filters out binary files and files matching any of the glob ignore patterns.
func FilterFiles(files []domain.DiffFile, ignorePatterns []string) []domain.DiffFile {
	filtered := make([]domain.DiffFile, 0, len(files))
	for _, f := range files {
		if f.IsBinary {
			continue
		}
		path := f.DisplayPath()
		if isIgnored(path, ignorePatterns) {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

func isIgnored(path string, patterns []string) bool {
	// Normalize path separators
	path = filepath.Clean(filepath.ToSlash(path))
	base := filepath.Base(path)

	for _, pattern := range patterns {
		pattern = filepath.Clean(filepath.ToSlash(pattern))
		// Exact base match or glob match on filename
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Match against full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Prefix directory match (e.g. "vendor/**")
		cleanPrefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(path, cleanPrefix+"/") || path == cleanPrefix {
			return true
		}
	}
	return false
}

// EstimateTokens provides a safe token count estimate for code diffs.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Conservative estimate for code: ~3 bytes per token
	tokens := (len(text) + 2) / 3
	return tokens
}

// FormatFilesDiff formats a slice of DiffFiles into unified diff text.
func FormatFilesDiff(files []domain.DiffFile) string {
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("## File: %s\n", f.DisplayPath()))
		if f.IsNew {
			sb.WriteString("*(New file)*\n")
		} else if f.IsDeleted {
			sb.WriteString("*(Deleted file)*\n")
		}
		if f.Patch != "" {
			sb.WriteString(f.Patch)
			if !strings.HasSuffix(f.Patch, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ChunkFiles partitions DiffFiles into chunks that each fit within maxTokensPerChunk.
func ChunkFiles(files []domain.DiffFile, maxTokensPerChunk int) [][]domain.DiffFile {
	if len(files) == 0 {
		return nil
	}
	if maxTokensPerChunk <= 0 {
		return [][]domain.DiffFile{files}
	}

	var chunks [][]domain.DiffFile
	var currentChunk []domain.DiffFile
	currentTokens := 0

	for _, f := range files {
		fileTokens := EstimateTokens(f.Patch) + EstimateTokens(f.DisplayPath()) + 20
		if len(currentChunk) > 0 && currentTokens+fileTokens > maxTokensPerChunk {
			chunks = append(chunks, currentChunk)
			currentChunk = []domain.DiffFile{f}
			currentTokens = fileTokens
		} else {
			currentChunk = append(currentChunk, f)
			currentTokens += fileTokens
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}
