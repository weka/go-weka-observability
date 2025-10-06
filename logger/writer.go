package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// infoLevels defines the log levels that should be written to the info writer
	infoLevels = []zerolog.Level{
		zerolog.TraceLevel,
		zerolog.DebugLevel,
		zerolog.InfoLevel,
	}
	// errorLevels defines the log levels that should be written to the error writer
	errorLevels = []zerolog.Level{
		zerolog.WarnLevel,
		zerolog.ErrorLevel,
		zerolog.FatalLevel,
		zerolog.PanicLevel,
	}
)

// SpecificLevelWriter routes logs to a writer based on log level
type SpecificLevelWriter struct {
	io.Writer
	Levels []zerolog.Level
}

// WriteLevel implements zerolog.LevelWriter interface
func (w SpecificLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if slices.Contains(w.Levels, level) {
		return w.Write(p)
	}
	return len(p), nil
}

// GetWriterFromFormat creates io.Writer based on format config
func GetWriterFromFormat(format FormatConfig) io.Writer {
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

// GetStderrWriter is DEPRECATED: Use GetWriterFromFormat instead.
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
	return GetWriterFromFormat(config)
}

// GetMultiLevelWriter is DEPRECATED: Use GetMultiLevelWriterWithConfig instead.
// This function uses environment variables for configuration.
// Migrate to explicit Config for better testability.
func GetMultiLevelWriter() io.Writer {
	config := NewDefaultConfigWithEnvOverrides()
	return GetMultiLevelWriterWithConfig(config)
}

// GetMultiLevelWriterWithConfig creates multi-level writer from complete config
func GetMultiLevelWriterWithConfig(config Config) io.Writer {
	// Validate FileMode sink configuration
	if config.Sink.Mode == FileMode {
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
	}

	infoWriter, errorWriter := createWritersForSink(config.Sink, config.Format)
	return createMultiLevelWriter(infoWriter, errorWriter)
}

// createWritersForSink determines writers based on sink mode
func createWritersForSink(sink SinkConfig, format FormatConfig) (info, error io.Writer) {
	if sink.Mode == FileMode {
		// File mode: separate files for info and error
		return createLumberjackWriter(sink, logLevelInfo),
			createLumberjackWriter(sink, logLevelError)
	}
	// Console mode: same writer for both (format-aware)
	writer := GetWriterFromFormat(format)
	return writer, writer
}

// createMultiLevelWriter assembles the multi-level writer with predefined level groups
func createMultiLevelWriter(infoWriter, errorWriter io.Writer) io.Writer {
	return zerolog.MultiLevelWriter(
		SpecificLevelWriter{Writer: infoWriter, Levels: infoLevels},
		SpecificLevelWriter{Writer: errorWriter, Levels: errorLevels},
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

// setCallerMarshalFunc configures caller directory display level
func setCallerMarshalFunc(callerDirLvl int) {
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
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

// Set the amount of nested dirs displayed before `<file_name>:<line_number>` for `caller` field in logger.
// `LOG_CALLER_DIR_LVL` is used for this.
// If unset - does nothing (default `caller` formatting is used)
// If `LOG_CALLER_DIR_LVL=0`, only the filename and line number are displayed (e.g. `message_processor.go:89`)
// see https://github.com/rs/zerolog/blob/master/README.md#add-file-and-line-number-to-log
//
// DEPRECATED: Use Config.Format.CallerDirLvl instead.
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

// GetLogLevel is DEPRECATED: Use Config.Format.Level instead.
// This function reads LOG_LEVEL environment variable directly.
// Migrate to structured configuration for better testability.
func GetLogLevel() zerolog.Level {
	lvlStr := os.Getenv("LOG_LEVEL")
	lvl := 1 // info level
	if val, err := strconv.Atoi(lvlStr); err == nil {
		lvl = val
	}
	return zerolog.Level(lvl)
}
