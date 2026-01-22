package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// ErrLogLevelOutOfBounds is returned when LOG_LEVEL value is outside valid range.
	ErrLogLevelOutOfBounds = errors.New("log level out of bounds")

	// ErrInvalidLogLevel is returned when LOG_LEVEL value is not a valid level name or number.
	ErrInvalidLogLevel = errors.New("invalid log level")

	// callerMarshalMutex protects global zerolog.CallerMarshalFunc modification
	callerMarshalMutex sync.Mutex
)

type (
	// LevelComparator is a function that determines if a log level should be written
	LevelComparator func(zerolog.Level) bool

	// SpecificLevelWriter routes logs to a writer based on log level comparison
	SpecificLevelWriter struct {
		io.Writer
		ShouldWrite LevelComparator
	}
)

// isInfoLevelOrBelow returns true for info-level and below (Trace, Debug, Info)
// These levels are routed to the info writer in file mode
func isInfoLevelOrBelow(level zerolog.Level) bool {
	return level < zerolog.WarnLevel
}

// isWarnLevelOrAbove returns true for warning-level and above (Warn, Error, Fatal, Panic)
// These levels are routed to the error writer in file mode
func isWarnLevelOrAbove(level zerolog.Level) bool {
	return level >= zerolog.WarnLevel
}

// WriteLevel implements zerolog.LevelWriter interface
func (w SpecificLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if w.ShouldWrite(level) {
		return w.Write(p)
	}

	return len(p), nil
}

// GetStderrWriterFromFormat creates stderr io.Writer based on format config.
// All outputs go to os.Stderr with formatting determined by the FormatConfig.
func GetStderrWriterFromFormat(format FormatConfig) io.Writer {
	timeFormat := time.RFC3339
	if format.TimeOnly {
		timeFormat = time.TimeOnly
	}

	switch format.Format {
	case LogFormatJSON:
		return os.Stderr
	case LogFormatPlain:
		return zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: timeFormat,
			NoColor:    true,
		}
	case LogFormatRaw:
		fallthrough
	default:
		return zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: timeFormat,
		}
	}
}

// GetStderrWriter creates stderr io.Writer based on environment variables.
//
// Deprecated: Use GetStderrWriterFromFormat with FormatConfig instead.
// This function reads LOG_FORMAT and LOG_TIME_ONLY directly from environment.
// Migrate to FormatConfig for better testability.
func GetStderrWriter() io.Writer {
	config := DefaultFormatConfig()
	// Override with env vars for backward compatibility
	if formatStr := os.Getenv("LOG_FORMAT"); formatStr != "" {
		if format, err := ParseLogFormat(formatStr); err != nil {
			slog.Warn("invalid LOG_FORMAT, using default", "error", err, "default", LogFormatJSON)
		} else {
			config.Format = format
		}
	}
	if os.Getenv("LOG_TIME_ONLY") == "true" {
		config.TimeOnly = true
	}

	return GetStderrWriterFromFormat(config)
}

// createConsoleMultiLevelWriter creates a multi-level writer for console output
// with stdout/stderr split based on format configuration
func createConsoleMultiLevelWriter(format FormatConfig) io.Writer {
	timeFormat := time.RFC3339
	if format.TimeOnly {
		timeFormat = time.TimeOnly
	}

	var stdoutWriter, stderrWriter io.Writer
	switch format.Format {
	case LogFormatJSON:
		stdoutWriter = os.Stdout
		stderrWriter = os.Stderr
	case LogFormatPlain:
		stdoutWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: timeFormat, NoColor: true}
		stderrWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: timeFormat, NoColor: true}
	case LogFormatRaw:
		fallthrough
	default:
		stdoutWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: timeFormat}
		stderrWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: timeFormat}
	}

	return createMultiLevelWriter(stdoutWriter, stderrWriter)
}

// GetMultiLevelWriter creates a multi-level writer that routes logs to stdout/stderr.
//
// Deprecated: Use GetStderrWriter for stderr-only output,
// or GetMultiLevelWriterWithConfig for file-based logging with level separation.
//
// This function maintains backward compatibility by routing logs to stdout/stderr:
// - Info and below (Trace, Debug, Info) → stdout
// - Warn and above (Warn, Error, Fatal, Panic) → stderr
//
// This behavior exists for backward compatibility but is not recommended for new code.
// Modern applications should use GetStderrWriter() which routes all logs to stderr.
func GetMultiLevelWriter() io.Writer {
	// Get format configuration from environment
	format := DefaultFormatConfig()
	if formatStr := os.Getenv("LOG_FORMAT"); formatStr != "" {
		if f, err := ParseLogFormat(formatStr); err != nil {
			slog.Warn("invalid LOG_FORMAT, using default", "error", err, "default", LogFormatJSON)
		} else {
			format.Format = f
		}
	}
	if os.Getenv("LOG_TIME_ONLY") == "true" {
		format.TimeOnly = true
	}

	return createConsoleMultiLevelWriter(format)
}

// GetMultiLevelWriterWithConfig creates writer from complete config
// Both ConsoleMode and FileMode use multi-level writers for level separation:
// - ConsoleMode: info/debug/trace → stdout, warn/error/fatal/panic → stderr
// - FileMode: info/debug/trace → info file, warn/error/fatal/panic → error file
//
//nolint:gocritic // hugeParam: intentional value semantics for clean API - called once at init, copy overhead negligible
func GetMultiLevelWriterWithConfig(config Config) io.Writer {
	if config.Sink.Mode == FileMode {
		// Validate FileMode sink configuration
		if config.Sink.FileName == "" {
			slog.Warn("FileMode requires FileName, using fallback",
				"fallback", defaultLogFileName)
			config.Sink.FileName = defaultLogFileName
		}
		if config.Sink.Dir == "" {
			fallbackDir := os.TempDir()
			slog.Warn("FileMode requires Dir, using fallback",
				"fallback", fallbackDir)
			config.Sink.Dir = fallbackDir
		}

		// FileMode: use multi-level writer for separate info/error files
		infoWriter := createLumberjackWriter(config.Sink, logLevelInfo)
		errorWriter := createLumberjackWriter(config.Sink, logLevelError)

		return createMultiLevelWriter(infoWriter, errorWriter)
	}

	// ConsoleMode: use multi-level writer with stdout/stderr split
	// (same behavior as GetMultiLevelWriter)
	return createConsoleMultiLevelWriter(config.Format)
}

// createMultiLevelWriter assembles the multi-level writer with level comparators
func createMultiLevelWriter(infoWriter, errorWriter io.Writer) io.Writer {
	return zerolog.MultiLevelWriter(
		SpecificLevelWriter{Writer: infoWriter, ShouldWrite: isInfoLevelOrBelow},
		SpecificLevelWriter{Writer: errorWriter, ShouldWrite: isWarnLevelOrAbove},
	)
}

// createLumberjackWriter creates a lumberjack writer for the specified level with automatic rotation
func createLumberjackWriter(sink SinkConfig, level string) io.Writer {
	// Create log file path
	logFileName := sink.FileName
	if level != logLevelInfo {
		// Add level suffix for error logs
		ext := filepath.Ext(logFileName)
		if ext != "" {
			base := logFileName[:len(logFileName)-len(ext)]
			logFileName = base + "-" + level + ext
		} else {
			logFileName = logFileName + "-" + level
		}
	}

	// Create lumberjack logger with rotation configuration
	return &lumberjack.Logger{
		Filename:   filepath.Join(sink.Dir, logFileName),
		MaxSize:    sink.MaxSizeMB, // megabytes
		MaxBackups: sink.MaxFiles,
		MaxAge:     sink.MaxAgeDays, // days
		Compress:   true,            // compress old files
		LocalTime:  true,            // use local time for timestamps
	}
}

// setCallerMarshalFunc configures caller directory display level.
// The mutex protects the global zerolog.CallerMarshalFunc from concurrent modifications.
// This ensures thread-safe updates when multiple goroutines might initialize loggers simultaneously.
func setCallerMarshalFunc(callerDirLvl int) {
	callerMarshalMutex.Lock()
	defer callerMarshalMutex.Unlock()

	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		short := file
		dirsNum := callerDirLvl
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				if dirsNum < 1 {
					break
				}
				dirsNum--
			}
		}

		return short + ":" + strconv.Itoa(line)
	}
}

// SetCallerDirDisplayLevel configures the amount of nested dirs displayed before
// `<file_name>:<line_number>` for `caller` field in logger using LOG_CALLER_DIR_LVL env var.
//
// If unset - does nothing (default `caller` formatting is used).
// If `LOG_CALLER_DIR_LVL=0`, only the filename and line number are displayed (e.g. `message_processor.go:89`).
// See https://github.com/rs/zerolog/blob/master/README.md#add-file-and-line-number-to-log
//
// Deprecated: Use Config.Format.CallerDirLvl instead.
// This function reads LOG_CALLER_DIR_LVL directly from environment.
// Migrate to FormatConfig for better testability.
func SetCallerDirDisplayLevel() {
	callerDirLvl, ok := os.LookupEnv("LOG_CALLER_DIR_LVL")
	if !ok {
		return
	}
	var lvl int
	if val, err := strconv.Atoi(callerDirLvl); err == nil {
		lvl = val
	}
	setCallerMarshalFunc(lvl)
}

// GetLogLevel returns the log level from LOG_LEVEL environment variable.
//
// Deprecated: Use Config.Format.Level instead.
// This function reads LOG_LEVEL environment variable directly.
// Migrate to structured configuration for better testability.
func GetLogLevel() zerolog.Level {
	lvlStr := os.Getenv("LOG_LEVEL")
	if lvlStr == "" {
		return zerolog.InfoLevel
	}

	level, err := parseLogLevel(lvlStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v, using default InfoLevel\n", err)

		return zerolog.InfoLevel
	}

	return level
}

// parseLogLevel parses a string to zerolog.Level.
// Supports both string names (info, debug, warn, error, trace, fatal, panic, disabled)
// and numeric values (-1=trace, 0=debug, 1=info, 2=warn, 3=error, 4=fatal, 5=panic, 7=disabled).
func parseLogLevel(s string) (zerolog.Level, error) {
	// First try parsing as a level name (case-insensitive)
	level, err := zerolog.ParseLevel(s)
	if err == nil {
		return level, nil
	}

	// Fall back to numeric parsing for backwards compatibility
	val, numErr := strconv.Atoi(s)
	if numErr != nil {
		return 0, fmt.Errorf("%w: %q is not a valid level name or number", ErrInvalidLogLevel, s)
	}

	if val < int(zerolog.TraceLevel) || val > int(zerolog.Disabled) {
		return 0, fmt.Errorf(
			"%w: LOG_LEVEL=%d (valid range: %d to %d)",
			ErrLogLevelOutOfBounds,
			val,
			int(zerolog.TraceLevel),
			int(zerolog.Disabled),
		)
	}

	return zerolog.Level(val), nil
}
