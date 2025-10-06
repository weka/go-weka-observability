package logger

import "fmt"

// OutputMode defines where logs should be written
type OutputMode string

const (
	// ConsoleMode outputs logs to stderr
	ConsoleMode OutputMode = "console"
	// FileMode outputs logs to files
	FileMode OutputMode = "file"
)

// ParseOutputMode parses a string into an OutputMode with validation
func ParseOutputMode(s string) (OutputMode, error) {
	switch s {
	case "console":
		return ConsoleMode, nil
	case "file":
		return FileMode, nil
	default:
		return ConsoleMode, fmt.Errorf("invalid output mode %q, valid options: console, file", s)
	}
}
