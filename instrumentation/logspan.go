package instrumentation

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	zerologger "github.com/weka/go-weka-observability/logger"
)

// Verbosity level constants for logr.Logger.V() method.
//
// IMPORTANT: Verbosity levels only affect logr.Info() calls, NOT logr.Error() calls.
// Error logs are ALWAYS logged regardless of verbosity level.
//
// These constants map to zerolog levels via the formula: zerologLevel = 1 - logrV
// The practical maximum for zerologr is V(2) since zerolog only has 3 main levels.
const (
	// VerbosityLevelInfo corresponds to zerolog.InfoLevel (logr.V(0) or logr.Info())
	// Default verbosity level - always logged unless verbosity is set lower than 0
	// Used for standard informational messages (Warn also uses this level)
	VerbosityLevelInfo = 0

	// VerbosityLevelDebug corresponds to zerolog.DebugLevel (logr.V(1))
	// Used for general debug information that can be filtered out in production
	VerbosityLevelDebug = 1

	// VerbosityLevelTrace corresponds to zerolog.TraceLevel (logr.V(2))
	// Used for detailed operation lifecycle tracing (span start/end, operation calls)
	// Most verbose level supported by zerologr
	VerbosityLevelTrace = 2

	// CallDepthOffset adjusts stack frame reporting for wrapped logger calls.
	// When logging through wrapper functions (SpanLogger methods, helper functions),
	// we skip 1 stack frame to report the actual caller's file:line instead of the wrapper's.
	//
	// Example without CallDepthOffset:
	//   func (ls *SpanLogger) Info(msg string) {
	//       ls.Logger.Info(msg)  // Log shows: logspan.go:171 > message
	//   }
	//
	// Example with CallDepthOffset:
	//   func (ls *SpanLogger) Info(msg string) {
	//       ls.Logger.WithCallDepth(CallDepthOffset).Info(msg)  // Log shows: caller.go:42 > message
	//   }
	CallDepthOffset = 1
)

func init() {
	// default global settings
	zerologr.VerbosityFieldName = ""
	zerologr.NameSeparator = "."
}

type ContextValuesKey struct{}

// SpanLogger is an abstract object that can be used instead of regular loggers and spans
type SpanLogger struct {
	Ctx context.Context
	logr.Logger
	trace.Span
}

// By default, log string in zerolog that uses `caller` will have formart:
// 2024-09-26T00:00:00+00:00 ERR path/to/file.go:217 > Error running some operation error="error text" additional_field=value logger=TopLevelName.NestedLoggerName
// without `caller`:
// 2024-09-26T00:00:00+00:00 ERR Error running some operation error="error text" additional_field=value logger=TopLevelName.NestedLoggerName
// ---
// This function will change the `logger` field to be put instead of `caller`:
// 2024-09-26T00:00:00+00:00 ERR TopLevelName.NestedLoggerName > Error running some operation error="error text" additional_field=value
func NewZerologrWithLoggerNameInsteadCaller() logr.Logger {
	initLogger := zerologger.NewZeroLoggerWithoutCaller()
	zerologr.NameFieldName = "caller"
	return zerologr.New(initLogger)
}

// Deprecated: Use logger.CreateLoggerFrom + logger.ContextWithLogr instead.
// This function has confusing triple behavior based on nil pointer checks:
//   - If baseLogger is nil and logger exists in context → reuses context logger
//   - If baseLogger is nil and no logger in context → creates new logger
//   - If baseLogger is not nil → uses provided logger
//
// Migration examples:
//
//	Old: ctx, logger := GetLoggerForContext(ctx, nil, "name", "key", "value")
//	New: logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverride())
//	     ctx = logger.ContextWithLogr(ctx, logr)
//	     logger := logger.MustLogrFromContext(ctx).WithName("name").WithValues("key", "value")
//
//	Old: ctx, logger := GetLoggerForContext(ctx, &existingLogger, "name")
//	New: ctx = logger.ContextWithLogr(ctx, existingLogger)
//	     logger := logger.MustLogrFromContext(ctx).WithName("name")
func GetLoggerForContext(ctx context.Context, baseLogger *logr.Logger, name string, keysAndValues ...any) (context.Context, logr.Logger) {
	var logger logr.Logger
	if baseLogger == nil {
		logger = zerologger.LogrFromContextOrDefault(ctx)
	} else {
		logger = *baseLogger
	}

	logger = logger.WithValues(keysAndValues...)
	if name != "" {
		logger = logger.WithName(name)
	}
	retCtx := zerologger.ContextWithLogr(ctx, logger)
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
		attrs = append(attrs, attributeFromValue(k, keysAndValues[i+1]))
	}
	return attrs
}

// attributeFromValue creates an OpenTelemetry attribute with proper type handling
func attributeFromValue(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	default:
		// Fallback to string representation for unknown types
		return attribute.String(key, fmt.Sprint(v))
	}
}

func (ls *SpanLogger) Info(msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(CallDepthOffset).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Debug(msg string, keysAndValues ...any) {
	// logr.V(1) is equivalent to zerolog.DebugLevel
	ls.V(1).WithCallDepth(CallDepthOffset).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Printf(msg string, args ...any) {
	ls.WithCallDepth(CallDepthOffset).Info(fmt.Sprintf(msg, args...))
}

func (ls *SpanLogger) Errorf(msg string, args ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(fmt.Errorf(msg, args...), "")
}

func (ls *SpanLogger) InfoWithStatus(code codes.Code, msg string, keysAnValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Info(msg, keysAnValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAnValues...)...)
	ls.AddEvent(msg)
	ls.SetStatus(code, msg)
}

func (ls *SpanLogger) Warn(msg string, keysAndValues ...any) {
	keysAndValues = append(keysAndValues, "level", "warn")
	ls.Logger.WithCallDepth(CallDepthOffset).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Error(err error, msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	ls.RecordError(err)
}

func (ls *SpanLogger) SetError(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	ls.SetStatus(codes.Error, msg)
	// TODO: Validate that error is not set yet
}

func (ls *SpanLogger) SetAttributes(attrs ...attribute.KeyValue) {
	if ls.Span != nil && len(attrs) > 0 {
		ls.Span.SetAttributes(attrs...)
	}
}

func (ls *SpanLogger) Fatal(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	os.Exit(1)
}

func (ls *SpanLogger) Panic(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	panic(err)
}

func (ls *SpanLogger) SetValues(keysAndValues ...any) {
	ls.Logger = ls.Logger.WithValues(keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
}

// validateGetLogSpanArgs ensures arguments are valid for GetLogSpan.
func validateGetLogSpanArgs(name string, keysAndValues []any) {
	if name == "" && len(keysAndValues) > 0 {
		panic("GetLogSpan must be called with no key/value pairs if name is empty")
	}
	if len(keysAndValues)%2 != 0 {
		panic("WithValues must be called with an even number of arguments")
	}
}

// getOrCreateLogger retrieves logger from context or creates a default one.
func getOrCreateLogger(ctx context.Context) logr.Logger {
	return zerologger.LogrFromContextOrDefault(ctx)
}

// enrichLogger adds name and key-value pairs to the logger.
func enrichLogger(logger logr.Logger, name string, keysAndValues []any) logr.Logger {
	if len(keysAndValues) > 0 {
		logger = logger.WithValues(keysAndValues...)
	}
	if name != "" {
		logger = logger.WithName(name)
	}
	return logger
}

// addTraceIDsIfValid adds trace and span IDs to logger if span is valid.
func addTraceIDsIfValid(logger logr.Logger, span trace.Span) logr.Logger {
	if span == nil {
		return logger
	}

	traceID := span.SpanContext().TraceID()
	spanID := span.SpanContext().SpanID()
	if traceID.IsValid() && spanID.IsValid() {
		logger = logger.WithValues("trace_id", traceID.String(), "span_id", spanID.String())
	}
	return logger
}

// createSpanShutdownFunc creates a shutdown function for the span.
func createSpanShutdownFunc(span trace.Span, logger logr.Logger, name string) func() {
	return func() {
		if span != nil && name != "" {
			span.End()
			logger.V(VerbosityLevelTrace).Info("span finished", "name", name)
		}
	}
}

// newSpanLogger creates a SpanLogger from logger and span.
func newSpanLogger(logger logr.Logger, span trace.Span) *SpanLogger {
	return &SpanLogger{
		Logger: logger,
		Span:   span,
	}
}

// logOperationStart logs that an operation has been called.
func logOperationStart(logger logr.Logger, name string) {
	logger.V(VerbosityLevelTrace).Info(name + " called")
}

// GetLogSpan creates or reuses a logger from context and creates a span for an operation.
// Returns context with logger, a SpanLogger combining logger and span, and a cleanup function.
func GetLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger, func()) {
	validateGetLogSpanArgs(name, keysAndValues)

	logger := getOrCreateLogger(ctx)
	logger = enrichLogger(logger, name, keysAndValues)
	ctx = zerologger.ContextWithLogr(ctx, logger)

	ctx, span := GetSpanForContext(ctx, name, keysAndValues...)
	logger = addTraceIDsIfValid(logger, span)

	shutdownFunc := createSpanShutdownFunc(span, logger, name)

	logOperationStart(logger, name)
	return ctx, newSpanLogger(logger, span), shutdownFunc
}
