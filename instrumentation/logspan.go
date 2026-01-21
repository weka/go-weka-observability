package instrumentation

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"go.opentelemetry.io/otel/attribute"
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

	// keyValuePairSize is the number of elements per key-value pair (key + value)
	// Used for capacity calculation when converting keysAndValues to attributes
	keyValuePairSize = 2
)

// globalZerologrInit is the singleton initializer for zerologr defaults.
//
//nolint:gochecknoglobals // singleton pattern - encapsulates sync.Once for thread-safe initialization
var globalZerologrInit = &zerologrInitializer{}

type (
	// ContextValuesKey is the context key type used to store span attribute values.
	ContextValuesKey struct{}

	// zerologrInitializer ensures zerologr defaults are set exactly once.
	zerologrInitializer struct {
		once sync.Once
	}
)

// initZerologrDefaults sets zerologr global settings.
// Called lazily via sync.Once when first logger is created.
func initZerologrDefaults() {
	globalZerologrInit.once.Do(func() {
		zerologr.VerbosityFieldName = ""
		zerologr.NameSeparator = "."
	})
}

// createChildSpan creates a new child span and enriches context with attributes.
// This is the core span creation logic used by CreateLogSpan.
//
// It merges keysAndValues with any previously stored context values (via ContextValuesKey),
// converts them to OpenTelemetry attributes, and stores the merged values back in context
// for future child spans to inherit.
//
// Returns:
//   - Updated context with new span and merged keysAndValues
//   - The created span
//   - Merged keysAndValues (original + inherited from context)
func createChildSpan(ctx context.Context, name string, keysAndValues []any) (context.Context, trace.Span, []any) {
	// Get tracer using smart resolution (context > cache > provider)
	tracer := GetTracer(ctx)

	// Start new child span
	ctx, span := tracer.Start(ctx, name)

	// Merge with values saved previously in context
	allKeysAndValues := keysAndValues
	if prevValues, ok := ctx.Value(ContextValuesKey{}).([]any); ok {
		// Create new slice with merged values (intentionally not modifying original keysAndValues)
		merged := make([]any, 0, len(keysAndValues)+len(prevValues))
		merged = append(merged, keysAndValues...)
		merged = append(merged, prevValues...)
		allKeysAndValues = merged
	}

	// Convert to span attributes and set them
	spanAttrs := getAttributesFromKeysAndValues(allKeysAndValues...)
	span.SetAttributes(spanAttrs...)

	// Store merged values in context for future spans
	ctx = context.WithValue(ctx, ContextValuesKey{}, allKeysAndValues)

	return ctx, span, allKeysAndValues
}

// createRootSpanInternal creates a new root span, breaking the parent chain.
// This is the core root span creation logic used by CreateRootLogSpan.
//
// Unlike createChildSpan, this does NOT merge with previous context values,
// as root spans are intentionally independent with fresh context.
//
// Returns:
//   - Updated context with new root span and stored keysAndValues
//   - The created root span
//
// nolint:spancheck // span ownership transferred to caller via SpanLogger.End()
func createRootSpanInternal(ctx context.Context, name string, keysAndValues []any) (context.Context, trace.Span) {
	// Get tracer using smart resolution (context > cache > provider)
	tracer := GetTracer(ctx)

	// Start new root span with WithNewRoot option
	ctx, span := tracer.Start(ctx, name, trace.WithNewRoot())

	// Convert to span attributes and set them (no merging for root spans)
	spanAttrs := getAttributesFromKeysAndValues(keysAndValues...)
	span.SetAttributes(spanAttrs...)

	// Store in context for future child spans
	ctx = context.WithValue(ctx, ContextValuesKey{}, keysAndValues)

	return ctx, span
}

// getCurrentSpan retrieves the current span from context without creating a new one.
// This is used when name is empty (reuse existing span pattern).
func getCurrentSpan(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func getAttributesFromKeysAndValues(keysAndValues ...any) []attribute.KeyValue {
	if len(keysAndValues)%keyValuePairSize != 0 {
		return []attribute.KeyValue{}
	}
	attrs := make([]attribute.KeyValue, 0, len(keysAndValues)/keyValuePairSize)
	for i := 0; i < len(keysAndValues); i += keyValuePairSize {
		k, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, attributeFromValue(k, keysAndValues[i+1]))
	}

	return attrs
}

// attributeFromValue creates an OpenTelemetry attribute with proper type handling.
// Scalar types are handled first, then slice types, with a fallback to string representation.
func attributeFromValue(key string, value any) attribute.KeyValue {
	if attr, ok := attributeFromScalar(key, value); ok {
		return attr
	}
	if attr, ok := attributeFromSlice(key, value); ok {
		return attr
	}
	// Fallback to string representation for unknown types
	return attribute.String(key, fmt.Sprint(value))
}

// attributeFromScalar handles scalar type conversion for OpenTelemetry attributes.
func attributeFromScalar(key string, value any) (attribute.KeyValue, bool) {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v), true
	case int:
		return attribute.Int(key, v), true
	case int64:
		return attribute.Int64(key, v), true
	case float64:
		return attribute.Float64(key, v), true
	case bool:
		return attribute.Bool(key, v), true
	default:
		return attribute.KeyValue{}, false
	}
}

// attributeFromSlice handles slice type conversion for OpenTelemetry attributes.
func attributeFromSlice(key string, value any) (attribute.KeyValue, bool) {
	switch v := value.(type) {
	case []string:
		return attribute.StringSlice(key, v), true
	case []int:
		return attribute.IntSlice(key, v), true
	case []int64:
		return attribute.Int64Slice(key, v), true
	case []float64:
		return attribute.Float64Slice(key, v), true
	case []bool:
		return attribute.BoolSlice(key, v), true
	default:
		return attribute.KeyValue{}, false
	}
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
// Ensures zerologr defaults are initialized on first call.
func getOrCreateLogger(ctx context.Context) logr.Logger {
	initZerologrDefaults()

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

// newSpanLogger creates a SpanLogger from context, logger and span.
func newSpanLogger(ctx context.Context, logger logr.Logger, span trace.Span, shutdown func()) *SpanLogger {
	return &SpanLogger{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: logger,
			Span:   span,
		},
		shutdown: shutdown,
	}
}

// logOperationStart logs that an operation has been called.
func logOperationStart(logger logr.Logger, name string) {
	logger.V(VerbosityLevelTrace).Info(name + " called")
}
