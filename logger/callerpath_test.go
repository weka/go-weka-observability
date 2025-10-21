package logger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weka/go-weka-observability/logger"
)

// TestCallerFilePath_KeepDepth tests the CallerFilePath type's KeepDepth method
func TestCallerFilePath_KeepDepth(t *testing.T) {
	tests := []struct {
		name    string
		pathStr string
		depth   int
		wantStr string
	}{
		{
			name:    "zero depth shows filename only",
			pathStr: "/path/to/project/pkg/file.go",
			depth:   0,
			wantStr: "file.go",
		},
		{
			name:    "depth 1 shows parent dir and filename",
			pathStr: "/path/to/project/pkg/file.go",
			depth:   1,
			wantStr: "pkg/file.go",
		},
		{
			name:    "depth 2 shows two parent dirs and filename",
			pathStr: "/path/to/project/pkg/sub/file.go",
			depth:   2,
			wantStr: "pkg/sub/file.go",
		},
		{
			name:    "depth 3 shows three parent dirs and filename",
			pathStr: "/home/user/go/src/project/internal/logger/writer.go",
			depth:   3,
			wantStr: "project/internal/logger/writer.go",
		},
		{
			name:    "excessive depth shows entire path",
			pathStr: "/short/path/file.go",
			depth:   100,
			wantStr: "/short/path/file.go",
		},
		{
			name:    "negative depth treated as zero",
			pathStr: "/path/to/file.go",
			depth:   -1,
			wantStr: "file.go",
		},
		{
			name:    "single file with leading slash and zero depth",
			pathStr: "/file.go",
			depth:   0,
			wantStr: "file.go",
		},
		{
			name:    "no separators returns unchanged",
			pathStr: "file.go",
			depth:   0,
			wantStr: "file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := logger.ParseCallerFilePath(tt.pathStr)
			got := path.KeepDepth(tt.depth)
			assert.Equal(t, tt.wantStr, got.String())
		})
	}
}

// TestCallerFilePath_WithLineNumber tests the CallerFilePath type's WithLineNumber method
func TestCallerFilePath_WithLineNumber(t *testing.T) {
	tests := []struct {
		name    string
		pathStr string
		line    int
		want    string
	}{
		{
			name:    "appends line number with colon",
			pathStr: "file.go",
			line:    42,
			want:    "file.go:42",
		},
		{
			name:    "handles path with directory",
			pathStr: "pkg/file.go",
			line:    100,
			want:    "pkg/file.go:100",
		},
		{
			name:    "handles large line numbers",
			pathStr: "file.go",
			line:    999999,
			want:    "file.go:999999",
		},
		{
			name:    "handles line number 1",
			pathStr: "test.go",
			line:    1,
			want:    "test.go:1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := logger.ParseCallerFilePath(tt.pathStr)
			got := path.WithLineNumber(tt.line)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseCallerFilePath_EmptyString tests edge case of empty string
func TestParseCallerFilePath_EmptyString(t *testing.T) {
	path := logger.ParseCallerFilePath("")
	assert.Equal(t, "", path.String())
}

// TestCallerFilePath_String tests the String method
func TestCallerFilePath_String(t *testing.T) {
	tests := []struct {
		name    string
		pathStr string
		want    string
	}{
		{
			name:    "absolute path",
			pathStr: "/path/to/file.go",
			want:    "/path/to/file.go",
		},
		{
			name:    "relative path",
			pathStr: "path/to/file.go",
			want:    "path/to/file.go",
		},
		{
			name:    "single file",
			pathStr: "file.go",
			want:    "file.go",
		},
		{
			name:    "empty string",
			pathStr: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := logger.ParseCallerFilePath(tt.pathStr)
			assert.Equal(t, tt.want, path.String())
		})
	}
}

// TestCallerFilePath_KeepDepth_EdgeCases tests edge cases that exercise takeLastParts bounds checking
func TestCallerFilePath_KeepDepth_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pathStr string
		depth   int
		wantStr string
	}{
		{
			name:    "empty path with zero depth returns empty",
			pathStr: "",
			depth:   0,
			wantStr: "",
		},
		{
			name:    "empty path with positive depth returns empty",
			pathStr: "",
			depth:   5,
			wantStr: "",
		},
		{
			name:    "empty path with negative depth returns empty",
			pathStr: "",
			depth:   -1,
			wantStr: "",
		},
		{
			name:    "single part with zero depth returns that part",
			pathStr: "file.go",
			depth:   0,
			wantStr: "file.go",
		},
		{
			name:    "single part with depth 1 returns that part",
			pathStr: "file.go",
			depth:   1,
			wantStr: "file.go",
		},
		{
			name:    "single part with excessive depth returns that part",
			pathStr: "file.go",
			depth:   100,
			wantStr: "file.go",
		},
		{
			name:    "very negative depth still returns filename",
			pathStr: "/path/to/file.go",
			depth:   -100,
			wantStr: "file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := logger.ParseCallerFilePath(tt.pathStr)
			got := path.KeepDepth(tt.depth)
			assert.Equal(t, tt.wantStr, got.String())
		})
	}
}
