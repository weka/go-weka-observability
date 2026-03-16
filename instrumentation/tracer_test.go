package instrumentation_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/instrumentation/oteltest"
)

// shutdownProvider is a helper to shutdown a TracerProvider and log any errors.
func shutdownProvider(ctx context.Context, t *testing.T, tp *trace.TracerProvider) {
	t.Helper()
	if err := tp.Shutdown(ctx); err != nil {
		t.Logf("failed to shutdown provider: %v", err)
	}
}

// shutdownRecorder is a helper to shutdown a SpanRecorder and log any errors.
func shutdownRecorder(ctx context.Context, t *testing.T, recorder *tracetest.SpanRecorder) {
	t.Helper()
	if err := recorder.Shutdown(ctx); err != nil {
		t.Logf("failed to shutdown recorder: %v", err)
	}
}

// TestGetTracerProviderDetection verifies that getTracer detects TracerProvider changes
// and automatically invalidates the cache when the provider is swapped.
func TestGetTracerProviderDetection(t *testing.T) {
	ctx := context.Background()

	// Create first provider
	recorder1 := tracetest.NewSpanRecorder()
	tp1 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder1))
	otel.SetTracerProvider(tp1)

	// First call - should create a span using tp1
	ctx1, span1Logger := instrumentation.CreateLogSpan(ctx, "test-op-1")
	span1Logger.End()

	spans1 := recorder1.Ended()
	require.Len(t, spans1, 1)
	assert.Equal(t, "test-op-1", spans1[0].Name())

	// Provider swap - create second provider
	recorder2 := tracetest.NewSpanRecorder()
	tp2 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder2))
	otel.SetTracerProvider(tp2)

	// Second call after provider swap - should detect change and use tp2
	ctx2, span2Logger := instrumentation.CreateLogSpan(ctx, "test-op-2")
	span2Logger.End()

	// Verify new span went to new recorder
	spans2 := recorder2.Ended()
	require.Len(t, spans2, 1)
	assert.Equal(t, "test-op-2", spans2[0].Name())

	// Original recorder should still have only one span
	assert.Len(t, recorder1.Ended(), 1)

	// Cleanup
	shutdownProvider(ctx1, t, tp1)
	shutdownProvider(ctx2, t, tp2)
}

// TestContextOverride verifies that context-based tracer injection takes
// priority over the global provider.
func TestContextOverride(t *testing.T) {
	ctx := context.Background()

	// Create global provider (will be ignored)
	_, globalRecorder := oteltest.SetupTesterWithProvider(context.Background())
	defer shutdownRecorder(context.Background(), t, globalRecorder)

	// Create custom tracer with context override
	customRecorder := tracetest.NewSpanRecorder()
	customTP := trace.NewTracerProvider(trace.WithSpanProcessor(customRecorder))
	customTracer := customTP.Tracer("custom-test-tracer")
	defer shutdownProvider(ctx, t, customTP)

	// Inject custom tracer via context
	ctx = instrumentation.ContextWithTracer(ctx, customTracer)

	// Create span - should use custom tracer
	_, spanLogger := instrumentation.CreateLogSpan(ctx, "custom-operation")
	spanLogger.End()

	// Verify span went to custom recorder, not global
	customSpans := customRecorder.Ended()
	require.Len(t, customSpans, 1)
	assert.Equal(t, "custom-operation", customSpans[0].Name())

	// Global recorder should have no spans
	assert.Empty(t, globalRecorder.Ended())
}

// TestProviderSwapPattern tests the provider swap pattern commonly used in tests.
// This mimics the existing test helper pattern from the design document.
func TestProviderSwapPattern(t *testing.T) {
	t.Parallel() // Safe with context-based isolation

	ctx := context.Background()

	// Use oteltest.SetupTester for parallel-safe test setup
	ctx, recorder := oteltest.SetupTester(ctx)
	defer shutdownRecorder(context.Background(), t, recorder)

	// GetTracer() resolves from context
	_, spanLogger := instrumentation.CreateLogSpan(ctx, "test-operation")
	spanLogger.Info("Test message")
	spanLogger.End()

	// Assertions
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-operation", spans[0].Name())
}

// TestConcurrentAccess verifies no race conditions when multiple goroutines
// access getTracer simultaneously. Run with -race flag.
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	// Use oteltest.SetupTesterWithProvider for simplified setup
	ctx, recorder := oteltest.SetupTesterWithProvider(ctx)
	defer shutdownRecorder(context.Background(), t, recorder)

	var wg sync.WaitGroup
	numGoroutines := 100

	for range numGoroutines {
		wg.Go(func() {
			_, spanLogger := instrumentation.CreateLogSpan(ctx, "concurrent-op")
			spanLogger.End()
		})
	}

	wg.Wait()

	// Verify all spans were created
	spans := recorder.Ended()
	assert.Len(t, spans, numGoroutines)
}

// TestContextOverrideWithRootSpan verifies context tracer injection works
// with CreateRootLogSpan as well.
func TestContextOverrideWithRootSpan(t *testing.T) {
	ctx := context.Background()

	// Create global provider (will be ignored)
	_, globalRecorder := oteltest.SetupTesterWithProvider(context.Background())
	defer shutdownRecorder(context.Background(), t, globalRecorder)

	// Create custom tracer
	customRecorder := tracetest.NewSpanRecorder()
	customTP := trace.NewTracerProvider(trace.WithSpanProcessor(customRecorder))
	customTracer := customTP.Tracer("custom-test-tracer")
	defer shutdownProvider(ctx, t, customTP)

	// Inject via context
	ctx = instrumentation.ContextWithTracer(ctx, customTracer)

	// Create root span
	_, rootLogger := instrumentation.CreateRootLogSpan(ctx, "root-operation", "job_id", "123")
	rootLogger.End()

	// Verify span went to custom recorder
	customSpans := customRecorder.Ended()
	require.Len(t, customSpans, 1)
	assert.Equal(t, "root-operation", customSpans[0].Name())

	// Global recorder should have no spans
	assert.Empty(t, globalRecorder.Ended())
}

// TestParallelTestsWithContextOverride demonstrates that multiple parallel tests
// can safely use context-based tracer injection for complete isolation.
func TestParallelTestsWithContextOverride(t *testing.T) {
	runTest := func(t *testing.T, testName string) {
		t.Parallel()

		ctx := context.Background()

		// Use oteltest.SetupTester for parallel-safe, isolated setup
		ctx, recorder := oteltest.SetupTester(ctx)
		defer shutdownRecorder(context.Background(), t, recorder)

		// Create span
		_, spanLogger := instrumentation.CreateLogSpan(ctx, testName)
		spanLogger.End()

		// Verify
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Equal(t, testName, spans[0].Name())
	}

	t.Run("test1", func(t *testing.T) { runTest(t, "operation-1") })
	t.Run("test2", func(t *testing.T) { runTest(t, "operation-2") })
	t.Run("test3", func(t *testing.T) { runTest(t, "operation-3") })
}

// TestCacheHitPerformance validates the fast path (cache hit) is used
// when provider doesn't change.
func TestCacheHitPerformance(t *testing.T) {
	ctx := context.Background()

	// Use oteltest.SetupTesterWithProvider for simplified setup
	ctx, recorder := oteltest.SetupTesterWithProvider(ctx)
	defer shutdownRecorder(context.Background(), t, recorder)

	// First call - cache miss
	_, span1 := instrumentation.CreateLogSpan(ctx, "op-1")
	span1.End()

	// Subsequent calls - cache hit (same provider)
	_, span2 := instrumentation.CreateLogSpan(ctx, "op-2")
	span2.End()

	_, span3 := instrumentation.CreateLogSpan(ctx, "op-3")
	span3.End()

	// All spans should be created successfully
	spans := recorder.Ended()
	assert.Len(t, spans, 3)
}

// TestProviderSwapInMiddleOfOperations tests that provider swap
// is detected even during ongoing operations.
func TestProviderSwapInMiddleOfOperations(t *testing.T) {
	ctx := context.Background()

	// Create first provider and span
	recorder1 := tracetest.NewSpanRecorder()
	tp1 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder1))
	otel.SetTracerProvider(tp1)

	ctx, span1 := instrumentation.CreateLogSpan(ctx, "before-swap")
	span1.End()

	// Swap provider mid-operation
	recorder2 := tracetest.NewSpanRecorder()
	tp2 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder2))
	otel.SetTracerProvider(tp2)

	// Create another span - should use new provider
	ctx, span2 := instrumentation.CreateLogSpan(ctx, "after-swap")
	span2.End()

	// Verify spans went to correct recorders
	assert.Len(t, recorder1.Ended(), 1)
	assert.Len(t, recorder2.Ended(), 1)

	// Cleanup
	shutdownProvider(ctx, t, tp1)
	shutdownProvider(ctx, t, tp2)
}

// TestSetupTester verifies the context-based test helper
func TestSetupTester(t *testing.T) {
	t.Parallel() // Safe

	ctx := context.Background()
	ctx, recorder := oteltest.SetupTester(ctx)
	defer shutdownRecorder(context.Background(), t, recorder)

	// Create span using helper context
	_, spanLogger := instrumentation.CreateLogSpan(ctx, "test-operation")
	spanLogger.End()

	// Verify span was recorded
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-operation", spans[0].Name())

	// Verify propagator was set (required for production code using propagation)
	propagator := otel.GetTextMapPropagator()
	assert.NotNil(t, propagator)
}

// TestSetupTesterParallelSafety verifies multiple parallel tests don't interfere
func TestSetupTesterParallelSafety(t *testing.T) {
	t.Parallel()

	for i := range 3 {
		t.Run(fmt.Sprintf("test-%d", i), func(t *testing.T) {
			t.Parallel() // Safe

			ctx := context.Background()
			ctx, recorder := oteltest.SetupTester(ctx)
			defer shutdownRecorder(context.Background(), t, recorder)

			// Each test has its own isolated recorder
			_, spanLogger := instrumentation.CreateLogSpan(ctx, fmt.Sprintf("op-%d", i))
			spanLogger.End()

			spans := recorder.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, fmt.Sprintf("op-%d", i), spans[0].Name())
		})
	}
}

// TestSetupTesterWithProvider verifies provider-based test helper
func TestSetupTesterWithProvider(t *testing.T) {
	// NO t.Parallel() - uses global state

	ctx := context.Background()
	ctx, recorder := oteltest.SetupTesterWithProvider(ctx)
	defer shutdownRecorder(context.Background(), t, recorder)

	// Verify provider was swapped
	_, spanLogger := instrumentation.CreateLogSpan(ctx, "test-operation")
	spanLogger.End()

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-operation", spans[0].Name())

	// Verify propagator was set (check it's not nil)
	propagator := otel.GetTextMapPropagator()
	assert.NotNil(t, propagator)
}

// ExampleSetupTester demonstrates parallel-safe test setup using context-based tracer injection.
// This is the recommended approach for tests that can run in parallel.
func ExampleSetupTester() {
	ctx := context.Background()

	// oteltest.SetupTester creates test environment with context-based tracer injection
	// Safe for parallel tests - uses ContextWithTracer internally
	ctx, recorder := oteltest.SetupTester(ctx)
	defer func() {
		if err := recorder.Shutdown(context.Background()); err != nil {
			_ = err // Example: cleanup error intentionally ignored
		}
	}()

	// Create spans using context-injected tracer
	_, logger := instrumentation.CreateLogSpan(ctx, "parallel-test-operation")
	logger.Info("Testing in parallel")
	logger.End()

	// Verify spans were recorded
	spans := recorder.Ended()
	fmt.Printf("Recorded %d span(s)\n", len(spans))
	fmt.Printf("Span name: %s\n", spans[0].Name())

	// Output:
	// Recorded 1 span(s)
	// Span name: parallel-test-operation
}

// ExampleSetupTesterWithProvider demonstrates sequential test setup using provider swap.
// This approach is simpler but NOT safe for parallel tests.
func ExampleSetupTesterWithProvider() {
	ctx := context.Background()

	// oteltest.SetupTesterWithProvider swaps global provider
	// NOT safe for parallel tests - must run sequentially
	ctx, recorder := oteltest.SetupTesterWithProvider(ctx)
	defer func() {
		if err := recorder.Shutdown(context.Background()); err != nil {
			_ = err // Example: cleanup error intentionally ignored
		}
	}()

	// Create spans - GetTracer() detects provider swap automatically
	_, logger := instrumentation.CreateLogSpan(ctx, "sequential-test-operation")
	logger.Info("Testing sequentially")
	logger.End()

	// Verify spans were recorded
	spans := recorder.Ended()
	fmt.Printf("Recorded %d span(s)\n", len(spans))
	fmt.Printf("Span name: %s\n", spans[0].Name())

	// Output:
	// Recorded 1 span(s)
	// Span name: sequential-test-operation
}

// ExampleContextWithTracer demonstrates custom tracer injection for advanced testing scenarios.
// Use this when you need fine-grained control over tracer configuration.
func ExampleContextWithTracer() {
	ctx := context.Background()

	// Create custom tracer with specific configuration
	recorder := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
	customTracer := tp.Tracer("custom-tracer")
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			_ = err // Example: cleanup error intentionally ignored
		}
	}()

	// Inject tracer via context - takes priority over global provider
	ctx = instrumentation.ContextWithTracer(ctx, customTracer)

	// Create spans using custom tracer
	_, logger := instrumentation.CreateLogSpan(ctx, "custom-tracer-operation")
	logger.Info("Using custom tracer")
	logger.End()

	// Verify spans were recorded
	spans := recorder.Ended()
	fmt.Printf("Recorded %d span(s)\n", len(spans))
	fmt.Printf("Span name: %s\n", spans[0].Name())

	// Output:
	// Recorded 1 span(s)
	// Span name: custom-tracer-operation
}
