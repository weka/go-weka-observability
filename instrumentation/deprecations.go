package instrumentation

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"go.opentelemetry.io/otel/trace"

	zerologger "github.com/weka/go-weka-observability/logger"
)

// Tracer is the global tracer instance.
//
// Deprecated: Use GetTracer(ctx) for direct tracer access, or preferably use the
// type-safe SpanLogger API: CreateSpan, CreateSpanWithOptions, CreateRootSpanWithOptions,
// or convenience functions (CreateServerSpan, CreateClientSpan, CreateProducerSpan,
// CreateConsumerSpan). Will be removed in v2.0.
//
// Reason: Global tracer doesn't support provider changes, test isolation, or context-based injection.
// The new trace management system with smart resolution is more flexible and test-friendly.
//
// # How Tracer Gets Set
//
// This variable is NOT set directly by SetupOTelSDKWithOptions(). Instead:
//
//  1. SetupOTelSDKWithOptions() calls otel.SetTracerProvider(provider)
//  2. When you first create a span (CreateSpan/CreateRootSpan/etc.), GetTracer(ctx) is called
//  3. GetTracer() creates a tracer from the provider and caches it
//  4. As part of caching, this Tracer variable is automatically synced for backward compatibility
//
// This lazy initialization pattern means:
//   - The provider is the source of truth (not this variable)
//   - Provider swaps are detected automatically (cache invalidates)
//   - Test tracer injection via ContextWithTracer() works seamlessly
//
// # Migration Guide
//
// Application code - Use type-safe SpanLogger API (recommended):
//   - OLD: ctx, span := instrumentation.Tracer.Start(ctx, "op")
//   - NEW (with key-values): ctx, logger := instrumentation.CreateLogSpan(ctx, "op", "key", "value")
//   - NEW (with options): ctx, logger := instrumentation.CreateLogSpanWithOptions(ctx, "op",
//     trace.WithSpanKind(trace.SpanKindClient),
//     trace.WithAttributes(attribute.String("key", "value")),
//     )
//   - NEW (convenience): ctx, logger := instrumentation.CreateClientSpan(ctx, "op",
//     trace.WithAttributes(attribute.String("key", "value")),
//     )
//   - NEW (convenience + key-values): ctx, logger := instrumentation.CreateClientSpan(ctx, "op")
//     ctx, logger = logger.WithValues("key", "value")  // Enrich logger and span
//
// Application code - Direct tracer access (advanced use only):
//   - OLD: instrumentation.Tracer.Start(ctx, "op", opts...)
//   - NEW: tracer := instrumentation.GetTracer(ctx)
//     ctx, span := tracer.Start(ctx, "op", opts...)
//
// Test code:
//   - Use ContextWithTracer(ctx, testTracer) for context-based injection
//   - OR use otel.SetTracerProvider(testProvider) for provider swap
//   - Both patterns are supported and automatically detected
//
// Direct assignment (instrumentation.Tracer = myTracer) will be overwritten on next span
// creation, so use ContextWithTracer() or otel.SetTracerProvider() instead.
var Tracer trace.Tracer

// Deprecated: Use logger.NewZerologrWithLoggerNameInsteadCaller instead.
//
// Reason: This function belongs in the logger package, not instrumentation package.
// Moving it improves package cohesion and clarity of responsibility.
//
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
// Reason: Confusing API with overloaded behavior based on nil pointer checks.
// The new API provides explicit functions for each use case (retrieve vs create),
// making code intent clearer and reducing cognitive load.
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
//
//	If your code calls GetLogSpan later, you MUST store the logger in context first:
//	  ctx = logger.ContextWithLogr(ctx, logr)
//
//	GetLogSpan retrieves the logger FROM CONTEXT. SetupOTelSDKWithOptions does NOT
//	do this automatically - it only uses the logger parameter for SDK initialization.
//
//	The order between ContextWithLogr() and SetupOTelSDKWithOptions() does NOT matter.
//	You only need to ensure ContextWithLogr() is called BEFORE GetLogSpan().
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

// GetSpanForContext creates or retrieves a span from context.
//
// Deprecated: Use CreateSpan, CreateSpanWithOptions, CreateRootSpanWithOptions,
// convenience functions (CreateServerSpan, CreateClientSpan, CreateProducerSpan,
// CreateConsumerSpan), or CurrentSpanLogger instead.
//
// Reason: Returns raw trace.Span without integrated logging, requires manual trace ID correlation,
// and has confusing empty-string overload. The new SpanLogger API unifies logging and tracing,
// provides compile-time safety (owned vs borrowed spans), and eliminates manual trace ID management.
//
// Migration:
//   - For child spans with key-values: instrumentation.CreateLogSpan(ctx, "operation", keysAndValues...)
//   - For child spans with options: instrumentation.CreateLogSpanWithOptions(ctx, "operation", opts...)
//   - For convenience functions: instrumentation.CreateClientSpan(ctx, "operation", opts...)
//   - For root spans: instrumentation.CreateRootLogSpan(ctx, "operation", keysAndValues...)
//   - For accessing current span: instrumentation.CurrentSpanLogger(ctx) or trace.SpanFromContext(ctx)
//
// The new SpanLogger API provides:
//   - Unified logging and tracing in a single type
//   - Compile-time safety (can't forget to call End() on owned spans)
//   - Clear ownership semantics (owned vs borrowed spans)
//   - Type-safe OpenTelemetry options via trace.SpanStartOption
//
// Migration examples:
//
//	// Old: Creating new spans
//	ctx, span := instrumentation.GetSpanForContext(ctx, "operation", "key", "value")
//	defer span.End()
//
//	// New: Use CreateSpan (with key-values)
//	ctx, logger := instrumentation.CreateLogSpan(ctx, "operation", "key", "value")
//	defer logger.End()  // Returns SpanLogger with integrated logging
//
//	// New: Use CreateSpanWithOptions (type-safe with OTel options)
//	ctx, logger := instrumentation.CreateLogSpanWithOptions(ctx, "operation",
//	    trace.WithSpanKind(trace.SpanKindClient),
//	    trace.WithAttributes(attribute.String("key", "value")),
//	)
//	defer logger.End()
//
//	// New: Use convenience functions
//	ctx, logger := instrumentation.CreateClientSpan(ctx, "operation",
//	    trace.WithAttributes(attribute.String("key", "value")),
//	)
//	defer logger.End()
//
//	// Old: Accessing current span
//	ctx, span := instrumentation.GetSpanForContext(ctx, "")
//
//	// New: Use CurrentSpanLogger or trace.SpanFromContext
//	view := instrumentation.CurrentSpanLogger(ctx)  // For logging + span
//	span := trace.SpanFromContext(ctx)              // For just the span
func GetSpanForContext(ctx context.Context, name string, keysAndValues ...any) (context.Context, trace.Span) {
	// Handle empty name case (reuse existing span)
	if name == "" {
		if len(keysAndValues) != 0 {
			panic("When re-using old context it is forbidden to modify span values, as new span is not created")
		}
		span := getCurrentSpan(ctx)
		return ctx, span
	}

	// Delegate to new API helper for span creation
	ctx, span, _ := createChildSpan(ctx, name, keysAndValues)
	return ctx, span
}

// GetLogSpan creates or reuses a logger from context and creates a span for an operation.
//
// Deprecated: Use CreateSpan, CreateSpanWithOptions, CreateRootSpanWithOptions,
// convenience functions (CreateServerSpan, CreateClientSpan, CreateProducerSpan,
// CreateConsumerSpan), or CurrentSpanLogger instead.
//
// Reason: Confusing empty-string overload (name="" reuses span, name="op" creates span).
// Returns separate logger and end() function instead of unified SpanLogger type.
// The new API provides clear, type-safe alternatives for each use case with better ergonomics.
//
// Migration Guide:
//
// CASE 1: Creating a new child span with key-values (name is not empty)
//
//	Old: ctx, logger, end := instrumentation.GetLogSpan(ctx, "operation", "key", "value")
//	     defer end()
//	New: ctx, logger := instrumentation.CreateLogSpan(ctx, "operation", "key", "value")
//	     defer logger.End()
//
// CASE 2: Creating a new child span with OpenTelemetry options (type-safe)
//
//	Old: // Not possible with GetLogSpan
//	New: ctx, logger := instrumentation.CreateLogSpanWithOptions(ctx, "operation",
//	         trace.WithSpanKind(trace.SpanKindClient),
//	         trace.WithAttributes(attribute.String("key", "value")),
//	     )
//	     defer logger.End()
//
// CASE 3: Creating spans with common span kinds (convenience functions)
//
//	Old: // Not possible with GetLogSpan
//	New (with OTel options): ctx, logger := instrumentation.CreateClientSpan(ctx, "operation",
//	         trace.WithAttributes(attribute.String("key", "value")),
//	     )
//	     defer logger.End()
//
//	New (with key-values): ctx, logger := instrumentation.CreateClientSpan(ctx, "operation")
//	     ctx, logger = logger.WithValues("key", "value")  // Enriched logger and span
//	     defer logger.End()
//
// CASE 4: Creating a root span (breaking parent chain)
//
//	Old: // Custom implementation with trace.WithNewRoot()
//	New: ctx, logger := instrumentation.CreateRootLogSpan(ctx, "operation", "key", "value")
//	     defer logger.End()
//
// CASE 5: Using current span without creating new one (name is empty string)
//
//	Old: _, logger, _ := instrumentation.GetLogSpan(ctx, "")
//	New: view := instrumentation.CurrentSpanLogger(ctx)
//
// IMPORTANT: Case 5 now returns *SpanLoggerView which cannot call End() (type safety).
// This prevents accidentally ending a span you don't own.
//
// Use CreateSpan for child spans with key-values, CreateSpanWithOptions for type-safe
// options, convenience functions for common span kinds, CreateRootSpan for independent
// traces, and CurrentSpanLogger for logging under the current span.
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
	return ctx, newSpanLogger(ctx, logger, span, shutdownFunc), shutdownFunc
}

// SetupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
// Additional resource attributes can be provided as key-value pairs.
//
// Deprecated: Use SetupOTelSDKWithOptions or SetupOTelSDKFrom instead.
//
// Reason: Doesn't support endpoint configuration via API (only env vars).
// The new functional options API provides flexible configuration with clear precedence
// (env overrides code) and better discoverability through named options.
//
// This function maintains backward compatibility but doesn't allow endpoint configuration via API.
//
// Migration examples:
//
// Old:
//
//	shutdown, err := instrumentation.SetupOTelSDK(ctx, "service", "v1", logger, "key", "value")
//
// New (functional options - recommended):
//
//	// Create logger with explicit options (overrideable via LOG_* env vars)
//	logr := logger.CreateLogger(
//	    logger.WithConsoleSink(),
//	    logger.WithInfoLevel(),
//	)
//	ctx = logger.ContextWithLogr(ctx, logr)
//
//	// Setup OpenTelemetry with options (OTEL_EXPORTER_OTLP_ENDPOINT env var can override)
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "service", "v1.0.0", logr,
//	    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
//	    instrumentation.WithResourceAttributes("key", "value"),
//	)
//
// New (explicit config):
//
//	config := instrumentation.NewDefaultOTelConfigWithEnvOverrides()
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "service", "v1", logr, config, "key", "value")
func SetupOTelSDK(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, keysAndValues ...any) (shutdown func(context.Context) error, err error) {
	return SetupOTelSDKWithOptions(
		ctx, serviceName, serviceVersion, logger,
		WithResourceAttributes(keysAndValues...),
	)
}
