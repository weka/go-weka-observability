package logger

import (
	"context"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type ctxLogger struct{}

type Logger struct {
	*zerolog.Logger
}

// Set the amount of nested dirs displayed before `<file_name>:<line_number>` for `caller` field in logger.
// `LOG_CALLER_DIR_LVL` is used for this.
// If unset - does nothing (default `caller` formatting is used)
// If `LOG_CALLER_DIR_LVL=0`, only the filename and line number are displayed (e.g. `message_processor.go:89`)
// see https://github.com/rs/zerolog/blob/master/README.md#add-file-and-line-number-to-log
func SetCallerDirDisplayLevel() {
	callerDirLvl, ok := os.LookupEnv("LOG_CALLER_DIR_LVL")
	if !ok {
		return
	}
	// get "caller" dir level value
	var lvl int // 0 by default (only file name will be displayed - with no dirs)
	if val, err := strconv.Atoi(callerDirLvl); err == nil {
		lvl = val
	}
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		dirsNum := lvl
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				if dirsNum < 1 {
					break
				}
				dirsNum--
			}
		}
		file = short
		return file + ":" + strconv.Itoa(line)
	}
}

type SpecificLevelWriter struct {
	io.Writer
	Levels []zerolog.Level
}

func (w SpecificLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	for _, l := range w.Levels {
		if l == level {
			return w.Write(p)
		}
	}
	return len(p), nil
}

func GetMultiLevelWriter() io.Writer {
	var stdOutWriter io.Writer
	var stdErrWriter io.Writer

	logFormat := os.Getenv("LOG_FORMAT")
	timeOnly := os.Getenv("LOG_TIME_ONLY")
	timeFormat := time.RFC3339

	if timeOnly == "true" {
		timeFormat = time.TimeOnly
	}

	if logFormat == "json" {
		stdOutWriter = os.Stdout
		stdErrWriter = os.Stderr
	} else {
		stdOutWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: timeFormat}
		stdErrWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: timeFormat}
	}

	writer := zerolog.MultiLevelWriter(
		SpecificLevelWriter{
			Writer: stdOutWriter,
			Levels: []zerolog.Level{
				zerolog.TraceLevel, zerolog.DebugLevel, zerolog.InfoLevel,
			},
		},
		SpecificLevelWriter{
			Writer: stdErrWriter,
			Levels: []zerolog.Level{
				zerolog.WarnLevel, zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel,
			},
		},
	)
	return writer
}

func GetLogLevel() zerolog.Level {
	lvlStr := os.Getenv("LOG_LEVEL")
	lvl := 1 // info level
	if val, err := strconv.Atoi(lvlStr); err == nil {
		lvl = val
	}
	return zerolog.Level(lvl)
}

func NewLogger() *Logger {
	log := NewZeroLogger()
	return &Logger{log}
}

func NewLoggerWithoutCaller() *Logger {
	log := NewZeroLoggerWithoutCaller()
	return &Logger{log}
}

func NewZeroLogger() *zerolog.Logger {
	log := zerolog.New(GetMultiLevelWriter()).
		Level(GetLogLevel()).
		With().
		Timestamp().
		Caller().
		Logger()
	return &log
}

func NewZeroLoggerWithoutCaller() *zerolog.Logger {
	log := zerolog.New(GetMultiLevelWriter()).
		Level(GetLogLevel()).
		With().
		Timestamp().
		Logger()
	return &log
}

func (l *Logger) Printf(msg string, opts ...any) {
	l.Info().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Warnf(msg string, opts ...any) {
	l.Warn().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Errorf(msg string, opts ...any) {
	l.Error().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Debugf(msg string, opts ...any) {
	l.Debug().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Tracef(msg string, opts ...any) {
	l.Trace().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) PrintErr(err error) {
	l.Info().CallerSkipFrame(1).Err(err).Send()
}

func (l *Logger) WarnErr(err error) {
	l.Warn().CallerSkipFrame(1).Err(err).Send()
}

func (l *Logger) FatalErr(err error) {
	l.Fatal().CallerSkipFrame(1).Err(err).Send()
}

func NewNamedLogger(serviceName string) *Logger {
	log := zerolog.New(GetMultiLevelWriter()).
		Level(GetLogLevel()).
		With().
		Str("service", serviceName).
		Timestamp().
		Caller().
		Logger()
	return &Logger{&log}
}

func GetLoggerFromContext(ctx context.Context) *Logger {
	if logger := ctx.Value(ctxLogger{}); logger != nil {
		ctxLogger, ok := logger.(*Logger)
		if ok {
			return ctxLogger
		}
	}
	return NewLogger()
}

func ContextWithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxLogger{}, l)
}

func GetLoggerFromExistingWithStrValues(logger *Logger, vals map[string]string) *Logger {
	newLoggerCtx := logger.With()
	for k, v := range vals {
		newLoggerCtx = newLoggerCtx.Str(k, v)
	}
	newLogger := newLoggerCtx.Logger()
	return &Logger{&newLogger}
}
