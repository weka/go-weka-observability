package instrumentation

import (
	"context"

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
// When you call CreateLogSpan() or CreateRootLogSpan(), GetTracer(ctx) checks:
//
//  1. Context for custom tracer (this function) ← Takes priority
//  2. Cached tracer from provider (normal path)
//
// This means context-based tracers always win over provider-based tracers.
//
// # Test Example
//
//	func TestWithCustomTracer(t *testing.T) {
//	    t.Parallel()  // Safe - no global state mutation
//
//	    // Create test tracer with span recorder
//	    recorder := tracetest.NewSpanRecorder()
//	    tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
//	    tracer := tp.Tracer("test-tracer")
//
//	    // Inject via context
//	    ctx := instrumentation.ContextWithTracer(context.Background(), tracer)
//
//	    // All CreateLogSpan/CreateRootLogSpan calls use injected tracer
//	    ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
//	    defer logger.End()
//
//	    // Verify spans were recorded
//	    spans := recorder.Ended()
//	    assert.Len(t, spans, 1)
//	}
//
// # Alternative: oteltest Package
//
// For simpler test setup, use the oteltest package which wraps this function:
//
//	ctx, recorder := oteltest.SetupTester(ctx)  // Parallel-safe
//	ctx, recorder := oteltest.SetupTesterWithProvider(ctx)  // Sequential tests
func ContextWithTracer(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, tracerKey{}, tracer)
}
