package parser

import (
	"regexp"
	"strconv"
	"strings"
)

// DiffFile represents a single file in a unified diff.
type DiffFile struct {
	Path  string
	Hunks []DiffHunk
}

// DiffHunk represents a single hunk in a unified diff.
type DiffHunk struct {
	NewStart int // starting line number in the new file
	NewCount int // number of lines in the new file side
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type   string // "add", "delete", "context"
	Number int    // line number in the new file (0 for deleted lines)
	Text   string
}

var (
	diffFileRe = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
	hunkRe     = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
)

// ParseUnifiedDiff parses a unified diff string into structured DiffFile entries.
func ParseUnifiedDiff(diff string) []DiffFile {
	var files []DiffFile
	var currentFile *DiffFile
	var currentHunk *DiffHunk
	newLineNum := 0

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		// New file
		if matches := diffFileRe.FindStringSubmatch(line); matches != nil {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				files = append(files, *currentFile)
			}
			currentFile = &DiffFile{Path: matches[2]}
			currentHunk = nil
			continue
		}

		// New hunk
		if matches := hunkRe.FindStringSubmatch(line); matches != nil {
			if currentHunk != nil && currentFile != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			start, _ := strconv.Atoi(matches[1])
			count := 1
			if matches[2] != "" {
				count, _ = strconv.Atoi(matches[2])
			}
			currentHunk = &DiffHunk{
				NewStart: start,
				NewCount: count,
			}
			newLineNum = start
			continue
		}

		// Skip file metadata lines
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "similarity") ||
			strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "Binary") {
			continue
		}

		if currentHunk == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "+"):
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:   "add",
				Number: newLineNum,
				Text:   line[1:],
			})
			newLineNum++
		case strings.HasPrefix(line, "-"):
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:   "delete",
				Number: 0,
				Text:   line[1:],
			})
		case strings.HasPrefix(line, " "):
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:   "context",
				Number: newLineNum,
				Text:   line[1:],
			})
			newLineNum++
		default:
			if strings.HasPrefix(line, `\`) {
				continue // Skip "\ No newline at end of file"
			}
			// Plain text or empty lines in a hunk — treat as context
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:   "context",
				Number: newLineNum,
				Text:   line,
			})
			newLineNum++
		}
	}

	// Flush remaining
	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		files = append(files, *currentFile)
	}

	return files
}

// ValidLines returns a set of (file, line) pairs that are valid targets for
// inline comments (added lines only).
func ValidLines(files []DiffFile) map[string]map[int]bool {
	result := make(map[string]map[int]bool)
	for _, f := range files {
		lineSet := make(map[int]bool)
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.Type == "add" && l.Number > 0 {
					lineSet[l.Number] = true
				}
			}
		}
		if len(lineSet) > 0 {
			result[f.Path] = lineSet
		}
	}
	return result
}
