package logger

import (
	"errors"
	"fmt"
)

const (
	// ConsoleMode outputs logs to stderr
	ConsoleMode OutputMode = "console"
	// FileMode outputs logs to files
	FileMode OutputMode = "file"
)

// ErrInvalidOutputMode is returned when an invalid output mode string is provided.
var ErrInvalidOutputMode = errors.New("invalid output mode")

// OutputMode defines where logs should be written
type OutputMode string

// ParseOutputMode parses a string into an OutputMode with validation
func ParseOutputMode(s string) (OutputMode, error) {
	switch s {
	case "console":
		return ConsoleMode, nil
	case "file":
		return FileMode, nil
	default:
		return ConsoleMode, fmt.Errorf("%w: %q, valid options: console, file", ErrInvalidOutputMode, s)
	}
}
