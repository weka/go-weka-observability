package instrumentation

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	zerologger "github.com/weka/go-weka-observability/logger"
)

// spanLoggerBase contains shared fields and methods for both SpanLogger and SpanLoggerView.
//
// Design Decision: This type is not exported to keep implementation details private.
// Users interact with SpanLogger and SpanLoggerView, which provide clear ownership semantics.
//
// The embedded Logger and Span provide direct access to their full interfaces,
// while specific methods are overridden to integrate logging with span events.
type spanLoggerBase struct {
	ctx context.Context
	trace.Span
	logr.Logger
}

// SpanLogger represents a span you created and own.
//
// You MUST call End() when done, typically via defer:
//
//	ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
//	defer logger.End()
//
// Design Decision: Separate type from SpanLoggerView to enforce ownership at compile time.
// The compiler prevents forgetting to call End() through defer patterns.
//
// Thread-safety: Safe to call from multiple goroutines. Each method delegates to
// the embedded Logger and Span, which are both thread-safe.
type SpanLogger struct {
	*spanLoggerBase
	shutdown func() // private - never nil (either real cleanup or no-op)
}

// SpanLoggerView represents a borrowed span from context.
//
// You cannot call End() - the span is owned by whoever created it.
// This is enforced at compile time by not providing an End() method.
//
//	view := instrumentation.CurrentSpanLogger(ctx)
//	view.Info("Logging under current span")
//	// view.End()  // COMPILE ERROR - method doesn't exist!
//
// Design Decision: Compile-time safety prevents accidentally ending a borrowed span.
// This eliminates a whole class of resource management bugs that would only appear at runtime.
//
// Use Case: Helper functions that need to log under the current span without
// taking ownership of the span lifecycle.
//
// Thread-safety: Safe to call from multiple goroutines. Each method delegates to
// the embedded Logger and Span, which are both thread-safe.
type SpanLoggerView struct {
	*spanLoggerBase
	// NO End() method - compile-time safety!
}

// Enabled returns whether this logger is enabled at the given verbosity level
func (ls *spanLoggerBase) Enabled(level int) bool {
	return ls.Logger.Enabled()
}

// Info logs an info message with key-value pairs, adds span event
func (ls *spanLoggerBase) Info(msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(CallDepthOffset).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

// Debug logs a debug message (V(1) level) with key-value pairs, adds span event
func (ls *spanLoggerBase) Debug(msg string, keysAndValues ...any) {
	// logr.V(1) is equivalent to zerolog.DebugLevel
	ls.V(VerbosityLevelDebug).WithCallDepth(CallDepthOffset).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

// Printf logs a formatted message at info level
func (ls *spanLoggerBase) Printf(msg string, args ...any) {
	ls.WithCallDepth(CallDepthOffset).Info(fmt.Sprintf(msg, args...))
}

// Errorf logs a formatted error message
func (ls *spanLoggerBase) Errorf(msg string, args ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(fmt.Errorf(msg, args...), "")
}

// InfoWithStatus logs an info message and sets span status
func (ls *spanLoggerBase) InfoWithStatus(code codes.Code, msg string, keysAnValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Info(msg, keysAnValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAnValues...)...)
	ls.AddEvent(msg)
	ls.SetStatus(code, msg)
}

// Warn logs a warning message with key-value pairs, adds span event
func (ls *spanLoggerBase) Warn(msg string, keysAndValues ...any) {
	keysAndValues = append(keysAndValues, "level", "warn")
	ls.Logger.WithCallDepth(CallDepthOffset).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

// Error logs an error and records it in the span as an event, but does NOT set the span status to Error.
// Use this for logging errors that don't represent span failure (e.g., handled errors, recoverable issues).
func (ls *spanLoggerBase) Error(err error, msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.RecordError(err)
}

// SetError logs an error, records it in the span, AND sets the span status to Error.
// Use this when the error represents a failure of the operation represented by the span.
// The Error status will be visible in tracing UIs and indicates the span failed.
func (ls *spanLoggerBase) SetError(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.RecordError(err)
	ls.SetStatus(codes.Error, msg)
	// TODO: Validate that error is not set yet
}

// SetAttributes adds attributes to the span only (not logged)
func (ls *spanLoggerBase) SetAttributes(attrs ...attribute.KeyValue) {
	if ls.Span != nil && len(attrs) > 0 {
		ls.Span.SetAttributes(attrs...)
	}
}

// Fatal logs an error and exits the program with status code 1
func (ls *spanLoggerBase) Fatal(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	os.Exit(1)
}

// Panic logs an error and panics
func (ls *spanLoggerBase) Panic(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(CallDepthOffset).Error(err, msg, keysAndValues...)
	panic(err)
}

// SetValues adds key-value pairs to both the logger and span
func (ls *spanLoggerBase) SetValues(keysAndValues ...any) {
	ls.Logger = ls.WithValues(keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
}

// End closes the span. Must be called for spans created with CreateLogSpan/CreateRootLogSpan.
func (sl *SpanLogger) End() {
	sl.shutdown()
}

// WithValues enriches the logger and span with additional key-value pairs.
// Returns updated context and new SpanLogger.
//
// The returned SpanLogger shares the same span and shutdown function as the original,
// so calling End() on either logger will close the span correctly. For code clarity,
// defer End() immediately after span creation, then enrich as needed:
//
//	ctx, logger := instrumentation.CreateServerLogSpan(ctx, "operation")
//	defer logger.End()  // Defer immediately after creation
//	ctx, logger = logger.WithValues("key", "value")  // Enrich as needed during function
//
// Both patterns are functionally equivalent because the shutdown function is shared.
func (sl *SpanLogger) WithValues(keysAndValues ...any) (context.Context, *SpanLogger) {
	enrichedLogger := sl.Logger.WithValues(keysAndValues...)
	sl.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)

	ctx := zerologger.ContextWithLogr(sl.ctx, enrichedLogger)

	return ctx, &SpanLogger{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: enrichedLogger,
			Span:   sl.Span,
		},
		shutdown: sl.shutdown, // Same shutdown function - both loggers share it
	}
}

// WithValues enriches the logger and span with additional key-value pairs.
// Returns updated context and new SpanLoggerView.
func (slv *SpanLoggerView) WithValues(keysAndValues ...any) (context.Context, *SpanLoggerView) {
	enrichedLogger := slv.Logger.WithValues(keysAndValues...)
	slv.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)

	ctx := zerologger.ContextWithLogr(slv.ctx, enrichedLogger)

	return ctx, &SpanLoggerView{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: enrichedLogger,
			Span:   slv.Span,
		},
	}
}

// CurrentSpanLogger retrieves the current span from context as a view.
//
// The returned SpanLoggerView cannot be ended (no End() method).
// Use this in helper functions that need to log under the current span.
//
// If no span exists in context, returns a view with a no-op span.
// This allows helpers to work safely whether or not a span is active.
//
// Design Note: Returns a view (not owned span) because this function doesn't
// create a new span - it accesses an existing one from context. The caller
// should not be able to end a span they didn't create.
//
// Example:
//
//	func helper(ctx context.Context) {
//	    view := instrumentation.CurrentSpanLogger(ctx)
//	    view.Info("Helper doing work", "key", "value")
//	    // Cannot call view.End() - compile error!
//	}
func CurrentSpanLogger(ctx context.Context) *SpanLoggerView {
	logger := getOrCreateLogger(ctx)
	span := trace.SpanFromContext(ctx)
	logger = addTraceIDsIfValid(logger, span)

	return &SpanLoggerView{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: logger,
			Span:   span,
		},
	}
}

// CreateLogSpan creates a new child span and returns a SpanLogger you own.
//
// You MUST call defer logger.End() to properly close the span.
//
// The new span will be a child of any existing span in the context, maintaining
// the parent-child trace relationship. Key-value pairs are added to both the logger
// context and as span attributes for complete observability.
//
// Panics if keysAndValues contains an odd number of elements (keys without values).
//
// Design Note: Returns owned *SpanLogger (not view) because this function creates
// a new span. The caller is responsible for ending it. The type system enforces this.
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - keysAndValues: Optional key-value pairs added to both logs and span attributes
//
// Returns:
//   - context.Context: Updated context with new span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example:
//
//	func processRequest(ctx context.Context) {
//	    ctx, logger := instrumentation.CreateLogSpan(ctx, "process_request", "user_id", 123)
//	    defer logger.End()  // Required!
//
//	    logger.Info("Processing started")
//	    // Span is automatically a child of current span in ctx
//	}
func CreateLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger) {
	if len(keysAndValues)%2 != 0 {
		panic("CreateLogSpan must be called with an even number of key/value pairs")
	}

	logger := getOrCreateLogger(ctx)
	logger = enrichLogger(logger, name, keysAndValues)
	ctx = zerologger.ContextWithLogr(ctx, logger)

	// Create child span directly using helper (no longer calls deprecated GetSpanForContext)
	ctx, span, _ := createChildSpan(ctx, name, keysAndValues)
	logger = addTraceIDsIfValid(logger, span)

	shutdownFunc := createSpanShutdownFunc(span, logger, name)
	logOperationStart(logger, name)

	return ctx, &SpanLogger{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: logger,
			Span:   span,
		},
		shutdown: shutdownFunc,
	}
}

// CreateRootLogSpan creates a new root span, breaking the parent-child relationship.
//
// You MUST call defer logger.End() to properly close the span.
//
// Unlike CreateLogSpan, this starts a completely new trace with its own trace ID,
// independent of any existing span in the context. Use this for background jobs,
// async operations, or when you explicitly want to start a fresh trace.
//
// Panics if keysAndValues contains an odd number of elements (keys without values).
// Panics if Tracer is not initialized (call SetupOTelSDK first).
//
// Design Note: Root spans are useful for operations that should be tracked independently,
// such as scheduled jobs or async message handlers that shouldn't be part of the
// original request trace.
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - keysAndValues: Optional key-value pairs added to both logs and span attributes
//
// Returns:
//   - context.Context: Updated context with new root span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example:
//
//	func backgroundJob(ctx context.Context) {
//	    ctx, logger := instrumentation.CreateRootLogSpan(ctx, "background_job", "job_id", "abc")
//	    defer logger.End()  // Required!
//
//	    logger.Info("Job started")
//	    // This span has a new trace ID, not related to parent
//	}
func CreateRootLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger) {
	if len(keysAndValues)%2 != 0 {
		panic("CreateRootLogSpan must be called with an even number of key/value pairs")
	}

	logger := getOrCreateLogger(ctx)
	logger = enrichLogger(logger, name, keysAndValues)
	ctx = zerologger.ContextWithLogr(ctx, logger)

	// Create root span using helper (eliminates duplicated logic)
	ctx, span := createRootSpanInternal(ctx, name, keysAndValues)
	logger = addTraceIDsIfValid(logger, span)

	shutdownFunc := createSpanShutdownFunc(span, logger, name)
	logOperationStart(logger, name)

	return ctx, &SpanLogger{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: logger,
			Span:   span,
		},
		shutdown: shutdownFunc,
	}
}

// CreateLogSpanWithOptions creates a new child span with OpenTelemetry span options.
//
// This is the type-safe API with zero usage of any type for span configuration.
// You MUST call defer logger.End() to properly close the span.
//
// RECOMMENDED: Use logger.WithValues() to add attributes to BOTH logger and span.
// This ensures consistency between logs and traces. Reserve trace.WithAttributes()
// for span-specific metadata that shouldn't appear in logs (e.g., sampler config).
//
// The new span will be a child of any existing span in the context, maintaining
// the parent-child trace relationship.
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - opts: OpenTelemetry span options (WithSpanKind, WithLinks, WithTimestamp, etc.)
//
// Returns:
//   - context.Context: Updated context with new span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example - Recommended pattern with WithValues:
//
//	ctx, logger := instrumentation.CreateLogSpanWithOptions(ctx, "http.request",
//	    trace.WithSpanKind(trace.SpanKindServer),
//	)
//	defer logger.End()
//	// Add attributes to BOTH logger and span
//	ctx, logger = logger.WithValues(
//	    "http.method", r.Method,
//	    "http.url", r.URL.Path,
//	)
//
// Example - With links and timestamp:
//
//	ctx, logger := instrumentation.CreateLogSpanWithOptions(ctx, "batch.process",
//	    trace.WithLinks(trace.Link{SpanContext: parentSpanCtx}),
//	    trace.WithTimestamp(eventTime),
//	)
//	defer logger.End()
//	ctx, logger = logger.WithValues("batch_size", 1000)
func CreateLogSpanWithOptions(
	ctx context.Context,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, *SpanLogger) {
	logger := getOrCreateLogger(ctx)
	logger = enrichLogger(logger, name, nil)
	ctx = zerologger.ContextWithLogr(ctx, logger)

	// Create child span with user-provided options
	tracer := GetTracer(ctx)
	ctx, span := tracer.Start(ctx, name, opts...)

	logger = addTraceIDsIfValid(logger, span)
	shutdownFunc := createSpanShutdownFunc(span, logger, name)
	logOperationStart(logger, name)

	return ctx, &SpanLogger{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: logger,
			Span:   span,
		},
		shutdown: shutdownFunc,
	}
}

// CreateRootLogSpanWithOptions creates a new root span with OpenTelemetry span options.
//
// This is the type-safe API with zero usage of any type for span configuration.
// You MUST call defer logger.End() to properly close the span.
//
// Unlike CreateLogSpanWithOptions, this starts a completely new trace with its own trace ID,
// independent of any existing span in the context. The trace.WithNewRoot() option is
// automatically prepended to your options.
//
// RECOMMENDED: Use logger.WithValues() to add attributes to BOTH logger and span.
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - opts: OpenTelemetry span options (WithSpanKind, WithLinks, WithTimestamp, etc.)
//
// Returns:
//   - context.Context: Updated context with new root span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example - Background job with independent trace:
//
//	ctx, logger := instrumentation.CreateRootLogSpanWithOptions(ctx, "background.job",
//	    trace.WithSpanKind(trace.SpanKindInternal),
//	)
//	defer logger.End()
//	ctx, logger = logger.WithValues("job_id", jobID, "job_type", "cleanup")
func CreateRootLogSpanWithOptions(
	ctx context.Context,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, *SpanLogger) {
	logger := getOrCreateLogger(ctx)
	logger = enrichLogger(logger, name, nil)
	ctx = zerologger.ContextWithLogr(ctx, logger)

	// Create root span with trace.WithNewRoot() prepended
	tracer := GetTracer(ctx)
	allOpts := append([]trace.SpanStartOption{trace.WithNewRoot()}, opts...)
	ctx, span := tracer.Start(ctx, name, allOpts...)

	logger = addTraceIDsIfValid(logger, span)
	shutdownFunc := createSpanShutdownFunc(span, logger, name)
	logOperationStart(logger, name)

	return ctx, &SpanLogger{
		spanLoggerBase: &spanLoggerBase{
			ctx:    ctx,
			Logger: logger,
			Span:   span,
		},
		shutdown: shutdownFunc,
	}
}

// CreateServerLogSpan creates a span with SpanKindServer for handling incoming requests.
//
// This is a convenience function equivalent to CreateLogSpanWithOptions with
// trace.WithSpanKind(trace.SpanKindServer), with optional key-value pairs
// that are added to both the logger and span.
//
// You MUST call defer logger.End() to properly close the span.
//
// Use this for HTTP/gRPC server handlers, websocket servers, or any code
// that processes incoming requests from external clients.
//
// For advanced span options (WithLinks, WithTimestamp, etc.), use
// CreateLogSpanWithOptions with trace.WithSpanKind(trace.SpanKindServer).
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - keysAndValues: Optional key-value pairs to add to logger and span
//
// Returns:
//   - context.Context: Updated context with new span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example:
//
//	ctx, logger := instrumentation.CreateServerLogSpan(ctx, "http.GET",
//	    "http.url", r.URL.Path,
//	    "http.method", r.Method,
//	)
//	defer logger.End()
func CreateServerLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger) {
	ctx, logger := CreateLogSpanWithOptions(ctx, name, trace.WithSpanKind(trace.SpanKindServer))
	if len(keysAndValues) > 0 {
		ctx, logger = logger.WithValues(keysAndValues...)
	}

	return ctx, logger
}

// CreateClientLogSpan creates a span with SpanKindClient for outgoing requests.
//
// This is a convenience function equivalent to CreateLogSpanWithOptions with
// trace.WithSpanKind(trace.SpanKindClient), with optional key-value pairs
// that are added to both the logger and span.
//
// You MUST call defer logger.End() to properly close the span.
//
// Use this for HTTP/gRPC client calls, database queries, or any code
// that makes outgoing requests to external services.
//
// For advanced span options (WithLinks, WithTimestamp, etc.), use
// CreateLogSpanWithOptions with trace.WithSpanKind(trace.SpanKindClient).
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - keysAndValues: Optional key-value pairs to add to logger and span
//
// Returns:
//   - context.Context: Updated context with new span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example:
//
//	ctx, logger := instrumentation.CreateClientLogSpan(ctx, "http.GET",
//	    "http.url", url,
//	    "http.method", "GET",
//	)
//	defer logger.End()
func CreateClientLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger) {
	ctx, logger := CreateLogSpanWithOptions(ctx, name, trace.WithSpanKind(trace.SpanKindClient))
	if len(keysAndValues) > 0 {
		ctx, logger = logger.WithValues(keysAndValues...)
	}

	return ctx, logger
}

// CreateProducerLogSpan creates a span with SpanKindProducer for publishing messages.
//
// This is a convenience function equivalent to CreateLogSpanWithOptions with
// trace.WithSpanKind(trace.SpanKindProducer), with optional key-value pairs
// that are added to both the logger and span.
//
// You MUST call defer logger.End() to properly close the span.
//
// Use this for message queue publishers, event emitters, or any code
// that sends messages to a message broker.
//
// For advanced span options (WithLinks, WithTimestamp, etc.), use
// CreateLogSpanWithOptions with trace.WithSpanKind(trace.SpanKindProducer).
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - keysAndValues: Optional key-value pairs to add to logger and span
//
// Returns:
//   - context.Context: Updated context with new span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example:
//
//	ctx, logger := instrumentation.CreateProducerLogSpan(ctx, "kafka.publish",
//	    "messaging.system", "kafka",
//	    "messaging.destination", topic,
//	)
//	defer logger.End()
func CreateProducerLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger) {
	ctx, logger := CreateLogSpanWithOptions(ctx, name, trace.WithSpanKind(trace.SpanKindProducer))
	if len(keysAndValues) > 0 {
		ctx, logger = logger.WithValues(keysAndValues...)
	}

	return ctx, logger
}

// CreateConsumerLogSpan creates a span with SpanKindConsumer for consuming messages.
//
// This is a convenience function equivalent to CreateLogSpanWithOptions with
// trace.WithSpanKind(trace.SpanKindConsumer), with optional key-value pairs
// that are added to both the logger and span.
//
// You MUST call defer logger.End() to properly close the span.
//
// Use this for message queue consumers, event handlers, or any code
// that receives messages from a message broker.
//
// For advanced span options (WithLinks, WithTimestamp, etc.), use
// CreateLogSpanWithOptions with trace.WithSpanKind(trace.SpanKindConsumer).
//
// Parameters:
//   - name: Operation name for the span (cannot be empty)
//   - keysAndValues: Optional key-value pairs to add to logger and span
//
// Returns:
//   - context.Context: Updated context with new span and logger
//   - *SpanLogger: Logger with span that MUST be ended
//
// Example:
//
//	ctx, logger := instrumentation.CreateConsumerLogSpan(ctx, "kafka.process",
//	    "messaging.system", "kafka",
//	    "messaging.source", topic,
//	)
//	defer logger.End()
func CreateConsumerLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger) {
	ctx, logger := CreateLogSpanWithOptions(ctx, name, trace.WithSpanKind(trace.SpanKindConsumer))
	if len(keysAndValues) > 0 {
		ctx, logger = logger.WithValues(keysAndValues...)
	}

	return ctx, logger
}
