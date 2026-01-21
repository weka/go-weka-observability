package instrumentation

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/weka/go-weka-observability/internal/version"
)

var (
	// Tracer cache with provider change detection
	cachedTracer   trace.Tracer
	cachedProvider trace.TracerProvider
	tracerCacheMu  sync.RWMutex
)

// tracerKey is the context key type for storing custom tracers
type tracerKey struct{}

// GetTracer returns the tracer instance for direct OpenTelemetry span creation.
//
// ⚠️ WARNING: Direct tracer usage bypasses SpanLogger integration (no automatic
// log/trace correlation, manual span.End() required, no compile-time safety).
// Consider using CreateLogSpanWithOptions() or convenience functions instead.
//
// # When to Use GetTracer
//
// Use this ONLY when you need direct access to the OpenTelemetry tracer API
// and cannot use CreateLogSpanWithOptions/CreateRootLogSpanWithOptions. Common reasons:
//   - Need to create spans without SpanLogger integration (performance-critical zero-allocation paths)
//   - Integration with third-party libraries expecting trace.Tracer
//   - Custom span lifecycle management not supported by SpanLogger
//   - Need OpenTelemetry span options not yet wrapped by this library
//
// For most use cases, prefer CreateLogSpanWithOptions with trace.SpanStartOption arguments.
//
// # Loss of SpanLogger Integration
//
// When using GetTracer, you lose:
//   - Automatic logger enrichment with trace IDs
//   - Unified logging + tracing via SpanLogger methods (Info, Error, etc.)
//   - Compile-time safety (must manually call span.End())
//   - Integration with logger.WithValues() for context enrichment
//
// You can still get a SpanLoggerView for the current span using CurrentSpanLogger(ctx).
//
// # Provider Integration
//
// This function integrates with OpenTelemetry's provider system:
//   - Reads from: otel.GetTracerProvider() (global provider set by SetupOTelSDKWithOptions)
//   - Creates: otel.Tracer(name, version) from the current provider
//   - Caches: Tracer instance with provider pointer for change detection
//   - Syncs: Updates deprecated instrumentation.Tracer variable for backward compatibility
//
// # Resolution Order
//
//  1. Context override: Check ctx.Value(tracerKey{}) for custom tracer (ContextWithTracer)
//  2. Cache hit: Return cached tracer if provider unchanged (fast path, RWMutex read)
//  3. Cache miss: Provider changed via otel.SetTracerProvider() - create new tracer
//
// # Performance
//
//   - Production (provider never changes): RWMutex read + pointer comparison (~10-50ns)
//   - Tests (provider swap detected): Write lock + create tracer (~1-10μs, rare)
//
// # Thread Safety
//
// Uses double-check locking pattern: RLock for fast read path, upgrades to Lock
// only when cache invalid. Safe for concurrent access from multiple goroutines.
//
// # Example - Advanced span creation:
//
//	tracer := instrumentation.GetTracer(ctx)
//	ctx, span := tracer.Start(ctx, "custom-operation",
//	    trace.WithSpanKind(trace.SpanKindInternal),
//	    trace.WithAttributes(attribute.String("key", "value")),
//	)
//	defer span.End()
//
//	// Can still get SpanLogger for current span
//	view := instrumentation.CurrentSpanLogger(ctx)
//	view.Info("Logging under custom span")
func GetTracer(ctx context.Context) trace.Tracer {
	// Priority 1: Context override (tests, multi-tenant scenarios)
	if tracer := ctx.Value(tracerKey{}); tracer != nil {
		return tracer.(trace.Tracer)
	}

	// Priority 2: Smart cached tracer with provider detection
	currentProvider := otel.GetTracerProvider()

	// Fast path: read-only check if cache is valid
	tracerCacheMu.RLock()
	if cachedTracer != nil && cachedProvider == currentProvider {
		defer tracerCacheMu.RUnlock()

		return cachedTracer
	}
	tracerCacheMu.RUnlock()

	// Slow path: cache miss or provider changed
	tracerCacheMu.Lock()
	defer tracerCacheMu.Unlock()

	// Double-check pattern (another goroutine might have updated)
	if cachedTracer != nil && cachedProvider == currentProvider {
		return cachedTracer
	}

	// Create new tracer from current provider
	cachedTracer = otel.Tracer(
		version.GetInstrumentationName(),
		trace.WithInstrumentationVersion(version.GetInstrumentationVersion()),
	)
	cachedProvider = currentProvider

	// Keep public Tracer in sync for legacy code
	Tracer = cachedTracer

	return cachedTracer
}
