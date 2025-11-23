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

// Deprecated: Use logger.LogrFromContextOrDefault or logger.CreateLogger instead.
//
// This function has confusing behavior based on nil pointer checks.
// The new API provides explicit functions for each use case.
//
// ┌─────────────────────────────────────────────────────────────────────────────┐
// │ CASE 1: baseLogger=nil → retrieves from context OR creates default         │
// └─────────────────────────────────────────────────────────────────────────────┘
//
// When you called GetLoggerForContext with nil baseLogger, it would:
// 1. Try to retrieve logger from context
// 2. If not found, create a default logger
//
// This is EXACTLY what LogrFromContextOrDefault does:
//
//	Old:
//	  ctx, logger := GetLoggerForContext(ctx, nil, "name")
//
//	New (simplest - direct equivalent):
//	  logger := logger.LogrFromContextOrDefault(ctx).WithName("name")
//
// If you're also calling SetupOTelSDK and need the logger in context for GetLogSpan:
//
//	Old:
//	  ctx, logger := GetLoggerForContext(ctx, nil, "name")
//	  shutdownFn, err := SetupOTelSDK(ctx, "service", "v1.0.0", logger)
//
//	New (if you want to CREATE a fresh logger and store it):
//	  logr := logger.CreateLogger()
//	  shutdownFn, err := instrumentation.SetupOTelSDKWithOptions(ctx, "service", "v1.0.0", logr)
//	  ctx = logger.ContextWithLogr(ctx, logr)          // Store for GetLogSpan later
//	  logger := logr.WithName("name")
//
//	New (if you want to RETRIEVE OR CREATE like the old behavior):
//	  logr := logger.LogrFromContextOrDefault(ctx)
//	  shutdownFn, err := instrumentation.SetupOTelSDKWithOptions(ctx, "service", "v1.0.0", logr)
//	  ctx = logger.ContextWithLogr(ctx, logr)          // Ensure it's stored for GetLogSpan
//	  logger := logr.WithName("name")
//
// ┌─────────────────────────────────────────────────────────────────────────────┐
// │ CASE 2: baseLogger provided (not nil) → uses provided logger               │
// └─────────────────────────────────────────────────────────────────────────────┘
//
//	Old:
//	  existingLogger := zerologr.New(logger.NewZeroLogger())
//	  ctx, logger := GetLoggerForContext(ctx, &existingLogger, "name")
//
//	New:
//	  logr := logger.CreateLogger()                    // Create logger
//	  ctx = logger.ContextWithLogr(ctx, logr)          // Store for GetLogSpan
//	  logger := logr.WithName("name")                  // Use it
//
// IMPORTANT: Why you might need logger.ContextWithLogr():
//   If your code calls GetLogSpan later, you MUST store the logger in context first:
//     ctx = logger.ContextWithLogr(ctx, logr)
//
//   GetLogSpan retrieves the logger FROM CONTEXT. SetupOTelSDKWithOptions does NOT
//   do this automatically - it only uses the logger parameter for SDK initialization.
//
//   The order between ContextWithLogr() and SetupOTelSDKWithOptions() does NOT matter.
//   You only need to ensure ContextWithLogr() is called BEFORE GetLogSpan().
//
// See docs/logger-initialization-migration.md for complete migration guide.
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

// Error logs an error and records it in the span as an event, but does NOT set the span status to Error.
// Use this for logging errors that don't represent span failure (e.g., handled errors, recoverable issues).
func (ls *SpanLogger) Error(err error, msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.RecordError(err)
}

// SetError logs an error, records it in the span, AND sets the span status to Error.
// Use this when the error represents a failure of the operation represented by the span.
// The Error status will be visible in tracing UIs and indicates the span failed.
func (ls *SpanLogger) SetError(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.RecordError(err)
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
// This is the primary function for combined logging and tracing in instrumented code.
//
// IMPORTANT: A logger MUST be stored in context before calling GetLogSpan, otherwise a
// default logger will be created. Always use logger.ContextWithLogr() to store your logger:
//
//	logr := logger.CreateLogger(logger.WithInfoLevel())
//	ctx = logger.ContextWithLogr(ctx, logr)  // REQUIRED!
//
// Returns:
//   - context.Context: Updated context with logger stored
//   - *SpanLogger: Combined logger and span that automatically enriches logs with trace IDs
//   - func(): Cleanup function that ends the span (must be called with defer)
//
// The SpanLogger automatically includes trace_id and span_id in all log messages,
// making it easy to correlate logs with traces in your observability backend.
//
// Parameters:
//   - name: Operation name for the span. Empty string ("") reuses the current span from context
//     without creating a new one. When name is empty, calling end() is safe (no-op), but not
//     recommended for code clarity - it makes it obvious the parent owns the span lifecycle.
//   - keysAndValues: Optional key-value pairs added to both logs and span attributes.
//     IMPORTANT: Cannot be used when name is empty (will panic).
//
// Example - Basic usage (creates new span):
//
//	func processRequest(ctx context.Context) {
//	    ctx, logger, end := instrumentation.GetLogSpan(ctx, "process_request")
//	    defer end() // MUST call end() when creating new span
//
//	    logger.Info("Processing started", "user_id", 123)
//	    // Logs include: trace_id=xxx span_id=yyy user_id=123
//	}
//
// Example - Nested operations (creates child spans):
//
//	func processRequest(ctx context.Context) {
//	    ctx, logger, end := instrumentation.GetLogSpan(ctx, "process_request")
//	    defer end()
//
//	    logger.Info("Processing started")
//	    queryDatabase(ctx) // This will be a child span
//	}
//
//	func queryDatabase(ctx context.Context) {
//	    ctx, logger, end := instrumentation.GetLogSpan(ctx, "query_database")
//	    defer end()
//
//	    logger.Info("Querying database")
//	    // This span is nested under process_request
//	}
//
// Example - With attributes (creates new span):
//
//	ctx, logger, end := instrumentation.GetLogSpan(ctx, "operation",
//	    "user_id", 123,
//	    "request_id", "req-456",
//	)
//	defer end()
//	// Both logs and span include user_id and request_id
//
// Example - Reusing parent span (NO new span created):
//
//	func helper(ctx context.Context) {
//	    // Get logger from context with current span's trace IDs
//	    _, logger, _ := instrumentation.GetLogSpan(ctx, "")
//	    // Calling end() here is safe (no-op) but not recommended for clarity
//	    // It's better to NOT call it to make it obvious parent owns the span
//
//	    logger.Info("Helper doing work")
//	    // Logs include parent's trace_id and span_id
//	}
//
// IMPORTANT: When name is empty:
//   - Returns the current span from context (doesn't create new one)
//   - Calling end() is safe (it's a no-op) but not recommended for code clarity
//   - Cannot pass keysAndValues (will panic)
//   - Use this for helper functions that should log under parent's span
//
// SpanLogger methods:
//   - Info(msg, keysAndValues...): Log at info level + add span event
//   - Debug(msg, keysAndValues...): Log at debug level + add span event
//   - Warn(msg, keysAndValues...): Log at warn level + add span event
//   - Error(err, msg, keysAndValues...): Log error + record error in span
//   - SetError(err, msg, keysAndValues...): Log error + set span status to error
//   - SetAttributes(attrs...): Add attributes to span only
//   - SetValues(keysAndValues...): Add to both logger and span
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
