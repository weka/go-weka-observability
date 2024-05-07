package instrumentation

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	setCallerDirDisplayLevel()
}

// Set the amount of nested dirs displayed before `<file_name>:<line_number>` for `caller` field in logger.
// `LOG_CALLER_DIR_LVL` is used for this.
// If unset - does nothing (default `caller` formatting is used)
// If `LOG_CALLER_DIR_LVL=0`, only the filename and line number are displayed (e.g. `message_processor.go:89`)
// see https://github.com/rs/zerolog/blob/master/README.md#add-file-and-line-number-to-log
func setCallerDirDisplayLevel() {
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

type ContextLoggerKey struct{}
type ContextValuesKey struct{}

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

func getWriter() io.Writer {
	var stdOutWriter io.Writer
	var stdErrWriter io.Writer

	logFormat := os.Getenv("LOG_FORMAT")

	if logFormat != "raw" {
		stdOutWriter = os.Stdout
		stdErrWriter = os.Stderr
	} else {
		stdOutWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		stdErrWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
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

func getLogLevel() zerolog.Level {
	lvlStr := os.Getenv("LOG_LEVEL")
	lvl := 1 // info level
	if val, err := strconv.Atoi(lvlStr); err == nil {
		lvl = val
	}
	return zerolog.Level(lvl)
}

func NewLogger() *zerolog.Logger {
	log := zerolog.New(getWriter()).
		Level(getLogLevel()).
		With().
		Timestamp().
		Caller().
		Logger()

	return &log
}

// SpanLogger is an abstract object that can be used instead of regular loggers and spans
type SpanLogger struct {
	Ctx context.Context
	logr.Logger
	trace.Span
	// spanName string // TODO: Do we need this?
}

func GetLoggerForContext(ctx context.Context, baseLogger *logr.Logger, name string, keysAndValues ...any) (context.Context, logr.Logger) {
	var logger logr.Logger
	if baseLogger == nil {
		if ctx.Value(ContextLoggerKey{}) != nil {
			logger = ctx.Value(ContextLoggerKey{}).(logr.Logger)
		} else {
			initLogger := NewLogger()
			logger = zerologr.New(initLogger)
		}
	} else {
		logger = *baseLogger
	}

	logger = logger.WithValues(keysAndValues...)
	if name != "" {
		logger = logger.WithName(name)
	}
	retCtx := context.WithValue(ctx, ContextLoggerKey{}, logger)
	return retCtx, logger
}

func GetSpanForContext(ctx context.Context, name string, keysAndValues ...any) (context.Context, trace.Span) {
	if Tracer == nil {
		panic("Tracer is not initialized. Call SetupOTelSDK first")
	}
	if name == "" {
		if len(keysAndValues) != 0 {
			panic("When re-using old context it is forbidden to modify span values, as new span is not created")
		}
		span := trace.SpanFromContext(ctx)
		return ctx, span
	}
	ctx, span := Tracer.Start(ctx, name)
	// expand with values saved previously in context
	if ctx.Value(ContextValuesKey{}) != nil {
		keysAndValues = append(keysAndValues, ctx.Value(ContextValuesKey{}).([]any)...)
	}
	spanAttrs := getAttributesFromKeysAndValues(keysAndValues...)
	span.SetAttributes(spanAttrs...)
	ctx = context.WithValue(ctx, ContextValuesKey{}, keysAndValues)
	return ctx, span
}

func (ls *SpanLogger) Enabled(level int) bool {
	return ls.Logger.Enabled()
}

func getAttributesFromKeysAndValues(keysAndValues ...any) []attribute.KeyValue {
	if len(keysAndValues)%2 != 0 {
		return []attribute.KeyValue{}
	}
	attrs := make([]attribute.KeyValue, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		k, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, attribute.String(k, fmt.Sprint(keysAndValues[i+1])))
	}
	return attrs
}

func (ls *SpanLogger) Info(msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(1).Info(msg, keysAndValues...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Debug(msg string, keysAndValues ...any) {
	// logr.V(1) is equivalent to zerolog.DebugLevel
	ls.V(1).WithCallDepth(1).Info(msg, keysAndValues...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Printf(msg string, args ...any) {
	ls.WithCallDepth(1).Info(fmt.Sprintf(msg, args...))
}

func (ls *SpanLogger) InfoWithStatus(code codes.Code, msg string, keysAnValues ...any) {
	ls.WithCallDepth(1).Info(msg, keysAnValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAnValues...)...)
	ls.SetStatus(code, msg)
}

func (ls *SpanLogger) Error(err error, msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(1).Error(err, msg, keysAndValues...)
	ls.RecordError(err)
}

func (ls *SpanLogger) SetError(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(1).Error(err, msg, keysAndValues...)
	ls.SetStatus(codes.Error, msg)
	// TODO: Validate that error is not set yet
}

func (ls *SpanLogger) SetAttributes(attrs ...attribute.KeyValue) {
	var keyvals []any
	for _, attr := range attrs {
		keyvals = append(keyvals, string(attr.Key))
		keyvals = append(keyvals, attr.Value.Emit())
	}
	if len(keyvals) > 0 && ls.Span != nil {
		ls.V(1).Info("Setting attributes", keyvals...)
		ls.Span.SetAttributes(attrs...)
	}
}

func (ls *SpanLogger) Fatal(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(1).Error(err, msg, keysAndValues...)
	os.Exit(1)
}

func (ls *SpanLogger) Panic(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(1).Error(err, msg, keysAndValues...)
	panic(err)
}

func (ls *SpanLogger) SetValues(keysAndValues ...any) {
	ls.Logger = ls.Logger.WithValues(keysAndValues...)
	if ls.Span != nil {
		ls.Span.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	}
}

func GetLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger, func()) {
	if name == "" && len(keysAndValues) > 0 {
		panic("GetLogSpan must be called with no key/value pairs if name is empty")
	}
	if len(keysAndValues)%2 != 0 {
		panic("WithValues must be called with an even number of arguments")
	}

	ctx, logger := GetLoggerForContext(ctx, nil, name, keysAndValues...)
	ctx, span := GetSpanForContext(ctx, name, keysAndValues...)

	if span != nil {
		traceID := span.SpanContext().TraceID()
		spanID := span.SpanContext().SpanID()
		if traceID.IsValid() && spanID.IsValid() {
			logger = logger.WithValues("trace_id", traceID.String(), "span_id", spanID.String())
		}
	}

	ShutdownFunc := func() {
		if span != nil && name != "" {
			span.End()
			logger.V(2).Info("span finished", "name", name)
		}
	}

	ls := SpanLogger{
		Logger: logger,
		Span:   span,
	}
	// logr.V(2) is equivalent to zerolog.TraceLevel
	logger.V(2).Info(fmt.Sprintf("%s called", name))
	return ctx, &ls, ShutdownFunc
}
