package logger

import (
	"errors"
	"fmt"
)

const (
	// LogFormatRaw outputs logs in human-readable format with colors
	LogFormatRaw LogFormat = "raw"
	// LogFormatJSON outputs logs in JSON format
	LogFormatJSON LogFormat = "json"
	// LogFormatPlain outputs logs in plain text format without colors
	LogFormatPlain LogFormat = "plain"
)

// ErrInvalidLogFormat is returned when an invalid log format string is provided.
var ErrInvalidLogFormat = errors.New("invalid log format")

// LogFormat represents the format of log output
type LogFormat string

// ParseLogFormat parses a string into a LogFormat with validation
func ParseLogFormat(s string) (LogFormat, error) {
	switch s {
	case "raw":
		return LogFormatRaw, nil
	case "json":
		return LogFormatJSON, nil
	case "plain":
		return LogFormatPlain, nil
	default:
		return LogFormatJSON, fmt.Errorf("%w: %q, valid options: raw, json, plain", ErrInvalidLogFormat, s)
	}
}
