package logger

import (
	"strconv"
	"strings"
)

// CallerFilePath represents a file path in caller information with pre-parsed parts.
// It provides O(1) operations for path manipulation by storing parts as a slice.
type CallerFilePath struct {
	parts []string // Directory and file parts, e.g. ["", "a", "b", "file.go"] for "/a/b/file.go"
}

// ParseCallerFilePath creates a CallerFilePath by splitting the file path on '/'.
// This pre-parsing allows efficient O(1) depth operations.
func ParseCallerFilePath(filePath string) CallerFilePath {
	parts := strings.Split(filePath, "/")
	return CallerFilePath{parts: parts}
}

// KeepDepth returns a new CallerFilePath with only the specified number of parent directories.
// depth of 0: returns only the filename
// depth of 1: returns one parent directory + filename
// Negative depths are treated as 0.
func (p CallerFilePath) KeepDepth(depth int) CallerFilePath {
	normalizedDepth := p.normalizeToMinimumDepth(depth)
	partsToKeep := p.calculatePartsToKeep(normalizedDepth)
	return p.takeLastParts(partsToKeep)
}

// normalizeToMinimumDepth ensures depth is at least 0 (minimum is filename only)
func (p CallerFilePath) normalizeToMinimumDepth(depth int) int {
	if depth < 0 {
		return 0
	}
	return depth
}

// calculatePartsToKeep determines how many path parts to keep.
// Formula: depth + 1 (the +1 accounts for the filename itself).
// Result is capped to the actual number of available parts.
func (p CallerFilePath) calculatePartsToKeep(depth int) int {
	partsToKeep := depth + 1
	if partsToKeep > len(p.parts) {
		return len(p.parts)
	}
	return partsToKeep
}

// takeLastParts returns a new CallerFilePath containing only the last n parts.
// If parts is empty, returns empty. The count is assumed to be valid (already normalized).
func (p CallerFilePath) takeLastParts(count int) CallerFilePath {
	if len(p.parts) == 0 {
		return p
	}

	start := len(p.parts) - count
	return CallerFilePath{parts: p.parts[start:]}
}

// String returns the string representation of the path.
func (p CallerFilePath) String() string {
	return strings.Join(p.parts, "/")
}

// WithLineNumber appends a line number to the path in the format "path:line".
func (p CallerFilePath) WithLineNumber(line int) string {
	return p.String() + ":" + strconv.Itoa(line)
}
