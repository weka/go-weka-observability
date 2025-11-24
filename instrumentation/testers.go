package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// ContextWithTracer stores a custom tracer in context.
//
// # Primary Use Case: Testing
//
// This is the recommended approach for tests requiring custom tracers with complete isolation.
// It overrides the provider-based tracer resolution, allowing tests to run in parallel without
// interfering with each other.
//
// # When To Use
//
//   - Tests: Inject test tracer with span recorder for verification
//   - Multi-tenant apps: Different tracing configuration per tenant (rare)
//   - Request-scoped customization: Override tracing behavior per request (very rare)
//
// # When NOT To Use (Use Provider Instead)
//
//   - Production code: Use SetupOTelSDKWithOptions() + otel.SetTracerProvider()
//   - Simple test provider swap: Use otel.SetTracerProvider(testProvider) - simpler
//   - Setting up tracing: This is for overriding, not initial setup
//
// # How It Works
//
// When you call CreateSpan() or CreateRootSpan(), GetTracer(ctx) checks:
//
//  1. Context for custom tracer (this function) ← Takes priority
//  2. Cached tracer from provider (normal path)
//
// This means context-based tracers always win over provider-based tracers.
//
// # Test Example
//
//	func TestWithCustomTracer(t *testing.T) {
//	    t.Parallel()  // ✅ Safe - no global state mutation
//
//	    // Create test tracer with span recorder
//	    recorder := tracetest.NewSpanRecorder()
//	    tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
//	    tracer := tp.Tracer("test-tracer")
//
//	    // Inject via context
//	    ctx := instrumentation.ContextWithTracer(context.Background(), tracer)
//
//	    // All CreateSpan/CreateRootSpan calls use injected tracer
//	    ctx, logger := instrumentation.CreateSpan(ctx, "operation")
//	    defer logger.End()
//
//	    // Verify spans were recorded
//	    spans := recorder.Ended()
//	    assert.Len(t, spans, 1)
//	}
//
// # Alternative Test Pattern (Provider Swap)
//
// For simpler tests, you can swap the provider instead:
//
//	otel.SetTracerProvider(testProvider)  // GetTracer() detects automatically
//	ctx, logger := instrumentation.CreateSpan(ctx, "operation")
//
// Both patterns work - use ContextWithTracer for parallel test isolation,
// use provider swap for simpler sequential tests.
func ContextWithTracer(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, tracerKey{}, tracer)
}

// SetupOTELTester creates a test context with tracer injection suitable for parallel tests.
// It returns a context with cached tracer and span recorder for assertions.
//
// This function uses context-based tracer injection (ContextWithTracer) for tracer isolation,
// but still sets the global TextMapPropagator since OpenTelemetry propagators are only
// available via global state. The propagator is set using sync.Once so it only happens once
// across all tests in the process. Parallel tests can safely use this function because:
//   - The propagator setup happens only once (sync.Once optimization)
//   - Tracer isolation is achieved via context (no cross-test interference)
//   - Production code relying on propagation will work correctly in tests
//
// # Usage
//
//	func TestMyFeature(t *testing.T) {
//	    t.Parallel()  // ✅ Safe for parallel execution
//
//	    ctx := context.Background()
//	    ctx, recorder := instrumentation.SetupOTELTester(ctx)
//	    defer recorder.Shutdown(context.Background())
//
//	    // Your test code using CreateSpan, CreateRootSpan, etc.
//	    ctx, logger := instrumentation.CreateSpan(ctx, "operation")
//	    defer logger.End()
//
//	    // Assertions
//	    spans := recorder.Ended()
//	    require.Len(t, spans, 1)
//	}
//
// # Returns
//
//   - ctx: Context with cached tracer injected via ContextWithTracer
//   - recorder: SpanRecorder for verifying spans in assertions
//
// # Cleanup
//
// Call recorder.Shutdown(ctx) when done to properly cleanup the TracerProvider.
// The SpanRecorder embeds the TracerProvider, so shutting down the recorder
// is sufficient.
func SetupOTELTester(ctx context.Context) (context.Context, *tracetest.SpanRecorder) {
	// Create span recorder for verification
	recorder := tracetest.NewSpanRecorder()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder))
	tracer := tp.Tracer("test-tracer")

	// Setup propagator once (thread-safe, only first call does the work)
	// Required for production code that uses trace propagation (HTTP headers, etc.)
	setupTestPropagatorOnce.Do(func() {
		prop := propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
		otel.SetTextMapPropagator(prop)
	})

	// Inject via context (provides tracer isolation for parallel tests)
	ctx = ContextWithTracer(ctx, tracer)

	return ctx, recorder
}

// SetupOTELTesterWithProvider creates a test environment by swapping the global TracerProvider
// and setting up TextMapPropagator for distributed tracing tests.
//
// ⚠️ WARNING: This swaps the global TracerProvider and TextMapPropagator which are shared
// across all goroutines. While the global state itself is thread-safe, parallel tests will
// interfere with each other by overriding each other's providers. Use SetupOTELTester for
// parallel test safety (context-based isolation).
//
// This is useful for:
//   - Sequential tests that need simpler setup
//   - Integration tests requiring TextMapPropagator (HTTP headers, distributed tracing)
//   - Tests that verify provider detection behavior
//
// # Usage
//
//	func TestDistributedTracing(t *testing.T) {
//	    // ⚠️ NO t.Parallel() - not safe with provider swap
//
//	    ctx := context.Background()
//	    ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
//	    defer recorder.Shutdown(context.Background())
//
//	    // Test code that uses propagation (HTTP headers, etc.)
//	    // CreateSpan will use the swapped provider automatically
//	}
//
// # Returns
//
//   - ctx: Input context returned as-is (tracer resolved from swapped provider)
//   - recorder: SpanRecorder for verifying spans in assertions
//
// # Propagator Setup
//
// This function sets otel.SetTextMapPropagator with TraceContext and Baggage propagators,
// matching the production setup in SetupOTelSDKInternal.
//
// # Cleanup
//
// Call recorder.Shutdown(ctx) when done. Note that this does NOT reset the global
// TracerProvider or TextMapPropagator - tests should run sequentially or clean up
// state between test runs.
func SetupOTELTesterWithProvider(ctx context.Context) (context.Context, *tracetest.SpanRecorder) {
	// Create span recorder
	recorder := tracetest.NewSpanRecorder()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder))

	// Swap global provider (NOT safe for parallel tests)
	otel.SetTracerProvider(tp)

	// Setup propagator (not using sync.Once - sequential tests may want different configs)
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	return ctx, recorder
}
