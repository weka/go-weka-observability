// Package oteltest provides test utilities for OpenTelemetry instrumentation.
//
// This package is intended for use in tests only. It provides helpers for
// setting up isolated tracing environments with span recording capabilities.
//
// # Quick Start
//
// For parallel tests (recommended):
//
//	func TestMyFeature(t *testing.T) {
//	    t.Parallel()
//	    ctx, recorder := oteltest.SetupTester(context.Background())
//	    defer recorder.Shutdown(context.Background())
//
//	    ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
//	    defer logger.End()
//
//	    spans := recorder.Ended()
//	    require.Len(t, spans, 1)
//	}
//
// For sequential tests (simpler setup):
//
//	func TestDistributedTracing(t *testing.T) {
//	    ctx, recorder := oteltest.SetupTesterWithProvider(context.Background())
//	    defer recorder.Shutdown(context.Background())
//
//	    // Test code using propagation...
//	}
package oteltest

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/weka/go-weka-observability/instrumentation"
)

// globalPropagatorInit is the singleton initializer.
//
//nolint:gochecknoglobals // singleton pattern - encapsulates sync.Once for one-time propagator setup
var globalPropagatorInit = &propagatorInitializer{}

// propagatorInitializer ensures propagator is set only once across all tests.
// Using a struct encapsulates the sync.Once pattern for cleaner architecture.
type propagatorInitializer struct {
	once sync.Once
}

// setup configures the global propagator if not already done.
func (pi *propagatorInitializer) setup() {
	pi.once.Do(func() {
		prop := propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
		otel.SetTextMapPropagator(prop)
	})
}

// SetupTester creates a test context with tracer injection suitable for parallel tests.
// It returns a context with injected tracer and a span recorder for assertions.
//
// This function uses context-based tracer injection (instrumentation.ContextWithTracer)
// for tracer isolation, but still sets the global TextMapPropagator since OpenTelemetry
// propagators are only available via global state. The propagator is set using sync.Once
// so it only happens once across all tests in the process. Parallel tests can safely use
// this function because:
//   - The propagator setup happens only once (sync.Once optimization)
//   - Tracer isolation is achieved via context (no cross-test interference)
//   - Production code relying on propagation will work correctly in tests
//
// # Usage
//
//	func TestMyFeature(t *testing.T) {
//	    t.Parallel()  // Safe for parallel execution
//
//	    ctx := context.Background()
//	    ctx, recorder := oteltest.SetupTester(ctx)
//	    defer recorder.Shutdown(context.Background())
//
//	    // Your test code using CreateLogSpan, CreateRootLogSpan, etc.
//	    ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
//	    defer logger.End()
//
//	    // Assertions
//	    spans := recorder.Ended()
//	    require.Len(t, spans, 1)
//	}
//
// # Returns
//
//   - ctx: Context with tracer injected via instrumentation.ContextWithTracer
//   - recorder: SpanRecorder for verifying spans in assertions
//
// # Cleanup
//
// Call recorder.Shutdown(ctx) when done to properly cleanup the TracerProvider.
// The SpanRecorder embeds the TracerProvider, so shutting down the recorder
// is sufficient.
func SetupTester(ctx context.Context) (context.Context, *tracetest.SpanRecorder) {
	// Create span recorder for verification
	recorder := tracetest.NewSpanRecorder()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(recorder))
	tracer := tp.Tracer("test-tracer")

	// Setup propagator once (thread-safe, only first call does the work)
	// Required for production code that uses trace propagation (HTTP headers, etc.)
	globalPropagatorInit.setup()

	// Inject via context (provides tracer isolation for parallel tests)
	ctx = instrumentation.ContextWithTracer(ctx, tracer)

	return ctx, recorder
}

// SetupTesterWithProvider creates a test environment by swapping the global TracerProvider
// and setting up TextMapPropagator for distributed tracing tests.
//
// WARNING: This swaps the global TracerProvider and TextMapPropagator which are shared
// across all goroutines. While the global state itself is thread-safe, parallel tests will
// interfere with each other by overriding each other's providers. Use SetupTester for
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
//	    // NO t.Parallel() - not safe with provider swap
//
//	    ctx := context.Background()
//	    ctx, recorder := oteltest.SetupTesterWithProvider(ctx)
//	    defer recorder.Shutdown(context.Background())
//
//	    // Test code that uses propagation (HTTP headers, etc.)
//	    // CreateLogSpan will use the swapped provider automatically
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
func SetupTesterWithProvider(ctx context.Context) (context.Context, *tracetest.SpanRecorder) {
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
