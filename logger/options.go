package logger

import "github.com/rs/zerolog"

// LoggerOption is a functional option for logger creation
type LoggerOption func(*Config)

// Sink options

// WithConsoleSink configures console output
func WithConsoleSink() LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = ConsoleMode
	}
}

// WithFileSink configures file output with rotation
func WithFileSink(dir, filename string) LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = FileMode
		c.Sink.Dir = dir
		c.Sink.FileName = filename
	}
}

// WithRotation configures log rotation parameters
func WithRotation(maxSizeMB, maxFiles, maxAgeDays int) LoggerOption {
	return func(c *Config) {
		c.Sink.MaxSizeMB = maxSizeMB
		c.Sink.MaxFiles = maxFiles
		c.Sink.MaxAgeDays = maxAgeDays
	}
}

// Format options

// WithJSONFormat configures JSON output
func WithJSONFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatJSON
	}
}

// WithRawFormat configures colored console output
func WithRawFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatRaw
	}
}

// WithPlainFormat configures plain text output (no colors)
func WithPlainFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatPlain
	}
}

// WithLevel sets the log level
func WithLevel(level zerolog.Level) LoggerOption {
	return func(c *Config) {
		c.Format.Level = level
	}
}

// WithDebugLevel convenience for debug logging
func WithDebugLevel() LoggerOption {
	return WithLevel(zerolog.DebugLevel)
}

// WithInfoLevel convenience for info logging
func WithInfoLevel() LoggerOption {
	return WithLevel(zerolog.InfoLevel)
}

// WithTraceLevel convenience for trace logging
func WithTraceLevel() LoggerOption {
	return WithLevel(zerolog.TraceLevel)
}

// WithWarnLevel convenience for warn logging
func WithWarnLevel() LoggerOption {
	return WithLevel(zerolog.WarnLevel)
}

// WithErrorLevel convenience for error logging
func WithErrorLevel() LoggerOption {
	return WithLevel(zerolog.ErrorLevel)
}

// WithCaller enables caller information with specified directory depth
func WithCaller(dirLevel int) LoggerOption {
	return func(c *Config) {
		c.Format.CallerDirLvl = dirLevel
	}
}

// WithTimeOnly uses time-only format instead of full timestamp
func WithTimeOnly() LoggerOption {
	return func(c *Config) {
		c.Format.TimeOnly = true
	}
}
