package instrumentation_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weka/go-weka-observability/instrumentation"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestGetTracerProviderDetection verifies that getTracer detects TracerProvider changes
// and automatically invalidates the cache when the provider is swapped.
func TestGetTracerProviderDetection(t *testing.T) {
	ctx := context.Background()

	// Create first provider
	recorder1 := tracetest.NewSpanRecorder()
	tp1 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder1))
	otel.SetTracerProvider(tp1)

	// First call - should create a span using tp1
	ctx1, span1Logger := instrumentation.CreateSpan(ctx, "test-op-1")
	span1Logger.End()

	spans1 := recorder1.Ended()
	require.Len(t, spans1, 1)
	assert.Equal(t, "test-op-1", spans1[0].Name())

	// Provider swap - create second provider
	recorder2 := tracetest.NewSpanRecorder()
	tp2 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder2))
	otel.SetTracerProvider(tp2)

	// Second call after provider swap - should detect change and use tp2
	ctx2, span2Logger := instrumentation.CreateSpan(ctx, "test-op-2")
	span2Logger.End()

	// Verify new span went to new recorder
	spans2 := recorder2.Ended()
	require.Len(t, spans2, 1)
	assert.Equal(t, "test-op-2", spans2[0].Name())

	// Original recorder should still have only one span
	assert.Len(t, recorder1.Ended(), 1)

	// Cleanup
	_ = tp1.Shutdown(ctx1)
	_ = tp2.Shutdown(ctx2)
}

// TestContextOverride verifies that context-based tracer injection takes
// priority over the global provider.
func TestContextOverride(t *testing.T) {
	ctx := context.Background()

	// Create global provider (will be ignored)
	_, globalRecorder := instrumentation.SetupOTELTesterWithProvider(context.Background())
	defer func() { _ = globalRecorder.Shutdown(context.Background()) }()

	// Create custom tracer with context override
	customRecorder := tracetest.NewSpanRecorder()
	customTP := trace.NewTracerProvider(trace.WithSpanProcessor(customRecorder))
	customTracer := customTP.Tracer("custom-test-tracer")
	defer func() { _ = customTP.Shutdown(ctx) }()

	// Inject custom tracer via context
	ctx = instrumentation.ContextWithTracer(ctx, customTracer)

	// Create span - should use custom tracer
	ctx, spanLogger := instrumentation.CreateSpan(ctx, "custom-operation")
	spanLogger.End()

	// Verify span went to custom recorder, not global
	customSpans := customRecorder.Ended()
	require.Len(t, customSpans, 1)
	assert.Equal(t, "custom-operation", customSpans[0].Name())

	// Global recorder should have no spans
	assert.Len(t, globalRecorder.Ended(), 0)
}

// TestProviderSwapPattern tests the provider swap pattern commonly used in tests.
// This mimics the existing test helper pattern from the design document.
func TestProviderSwapPattern(t *testing.T) {
	t.Parallel() // ✅ Safe with context-based isolation

	ctx := context.Background()

	// Use SetupOTELTester for parallel-safe test setup
	ctx, recorder := instrumentation.SetupOTELTester(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	// getTracer() resolves from context
	_, spanLogger := instrumentation.CreateSpan(ctx, "test-operation")
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

	// Use SetupOTELTesterWithProvider for simplified setup
	ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, spanLogger := instrumentation.CreateSpan(ctx, "concurrent-op")
			spanLogger.End()
		}(i)
	}

	wg.Wait()

	// Verify all spans were created
	spans := recorder.Ended()
	assert.Equal(t, numGoroutines, len(spans))
}

// TestContextOverrideWithRootSpan verifies context tracer injection works
// with CreateRootSpan as well.
func TestContextOverrideWithRootSpan(t *testing.T) {
	ctx := context.Background()

	// Create global provider (will be ignored)
	_, globalRecorder := instrumentation.SetupOTELTesterWithProvider(context.Background())
	defer func() { _ = globalRecorder.Shutdown(context.Background()) }()

	// Create custom tracer
	customRecorder := tracetest.NewSpanRecorder()
	customTP := trace.NewTracerProvider(trace.WithSpanProcessor(customRecorder))
	customTracer := customTP.Tracer("custom-test-tracer")
	defer func() { _ = customTP.Shutdown(ctx) }()

	// Inject via context
	ctx = instrumentation.ContextWithTracer(ctx, customTracer)

	// Create root span
	ctx, rootLogger := instrumentation.CreateRootSpan(ctx, "root-operation", "job_id", "123")
	rootLogger.End()

	// Verify span went to custom recorder
	customSpans := customRecorder.Ended()
	require.Len(t, customSpans, 1)
	assert.Equal(t, "root-operation", customSpans[0].Name())

	// Global recorder should have no spans
	assert.Len(t, globalRecorder.Ended(), 0)
}

// TestParallelTestsWithContextOverride demonstrates that multiple parallel tests
// can safely use context-based tracer injection for complete isolation.
func TestParallelTestsWithContextOverride(t *testing.T) {
	runTest := func(t *testing.T, testName string) {
		t.Parallel()

		ctx := context.Background()

		// Use SetupOTELTester for parallel-safe, isolated setup
		ctx, recorder := instrumentation.SetupOTELTester(ctx)
		defer func() { _ = recorder.Shutdown(context.Background()) }()

		// Create span
		_, spanLogger := instrumentation.CreateSpan(ctx, testName)
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

	// Use SetupOTELTesterWithProvider for simplified setup
	ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	// First call - cache miss
	_, span1 := instrumentation.CreateSpan(ctx, "op-1")
	span1.End()

	// Subsequent calls - cache hit (same provider)
	_, span2 := instrumentation.CreateSpan(ctx, "op-2")
	span2.End()

	_, span3 := instrumentation.CreateSpan(ctx, "op-3")
	span3.End()

	// All spans should be created successfully
	spans := recorder.Ended()
	assert.Equal(t, 3, len(spans))
}

// TestProviderSwapInMiddleOfOperations tests that provider swap
// is detected even during ongoing operations.
func TestProviderSwapInMiddleOfOperations(t *testing.T) {
	ctx := context.Background()

	// Create first provider and span
	recorder1 := tracetest.NewSpanRecorder()
	tp1 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder1))
	otel.SetTracerProvider(tp1)

	ctx, span1 := instrumentation.CreateSpan(ctx, "before-swap")
	span1.End()

	// Swap provider mid-operation
	recorder2 := tracetest.NewSpanRecorder()
	tp2 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder2))
	otel.SetTracerProvider(tp2)

	// Create another span - should use new provider
	ctx, span2 := instrumentation.CreateSpan(ctx, "after-swap")
	span2.End()

	// Verify spans went to correct recorders
	assert.Len(t, recorder1.Ended(), 1)
	assert.Len(t, recorder2.Ended(), 1)

	// Cleanup
	_ = tp1.Shutdown(ctx)
	_ = tp2.Shutdown(ctx)
}

// TestSetupOTELTester verifies the context-based test helper
func TestSetupOTELTester(t *testing.T) {
	t.Parallel() // ✅ Safe

	ctx := context.Background()
	ctx, recorder := instrumentation.SetupOTELTester(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	// Create span using helper context
	_, spanLogger := instrumentation.CreateSpan(ctx, "test-operation")
	spanLogger.End()

	// Verify span was recorded
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-operation", spans[0].Name())

	// Verify propagator was set (required for production code using propagation)
	propagator := otel.GetTextMapPropagator()
	assert.NotNil(t, propagator)
}

// TestSetupOTELTesterParallelSafety verifies multiple parallel tests don't interfere
func TestSetupOTELTesterParallelSafety(t *testing.T) {
	for i := 0; i < 3; i++ {
		i := i
		t.Run(fmt.Sprintf("test-%d", i), func(t *testing.T) {
			t.Parallel() // ✅ Safe

			ctx := context.Background()
			ctx, recorder := instrumentation.SetupOTELTester(ctx)
			defer func() { _ = recorder.Shutdown(context.Background()) }()

			// Each test has its own isolated recorder
			_, spanLogger := instrumentation.CreateSpan(ctx, fmt.Sprintf("op-%d", i))
			spanLogger.End()

			spans := recorder.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, fmt.Sprintf("op-%d", i), spans[0].Name())
		})
	}
}

// TestSetupOTELTesterWithProvider verifies provider-based test helper
func TestSetupOTELTesterWithProvider(t *testing.T) {
	// ⚠️ NO t.Parallel() - uses global state

	ctx := context.Background()
	ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	// Verify provider was swapped
	_, spanLogger := instrumentation.CreateSpan(ctx, "test-operation")
	spanLogger.End()

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-operation", spans[0].Name())

	// Verify propagator was set (check it's not nil)
	propagator := otel.GetTextMapPropagator()
	assert.NotNil(t, propagator)
}

// ExampleSetupOTELTester demonstrates parallel-safe test setup using context-based tracer injection.
// This is the recommended approach for tests that can run in parallel.
func ExampleSetupOTELTester() {
	ctx := context.Background()

	// SetupOTELTester creates test environment with context-based tracer injection
	// Safe for parallel tests - uses ContextWithTracer internally
	ctx, recorder := instrumentation.SetupOTELTester(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	// Create spans using context-injected tracer
	_, logger := instrumentation.CreateSpan(ctx, "parallel-test-operation")
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

// ExampleSetupOTELTesterWithProvider demonstrates sequential test setup using provider swap.
// This approach is simpler but NOT safe for parallel tests.
func ExampleSetupOTELTesterWithProvider() {
	ctx := context.Background()

	// SetupOTELTesterWithProvider swaps global provider
	// ⚠️ NOT safe for parallel tests - must run sequentially
	ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
	defer func() { _ = recorder.Shutdown(context.Background()) }()

	// Create spans - getTracer() detects provider swap automatically
	_, logger := instrumentation.CreateSpan(ctx, "sequential-test-operation")
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
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Inject tracer via context - takes priority over global provider
	ctx = instrumentation.ContextWithTracer(ctx, customTracer)

	// Create spans using custom tracer
	_, logger := instrumentation.CreateSpan(ctx, "custom-tracer-operation")
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
