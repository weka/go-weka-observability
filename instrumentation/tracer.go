package instrumentation

import (
	"context"
	"sync"

	"github.com/weka/go-weka-observability/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Tracer cache with provider change detection
	cachedTracer   trace.Tracer
	cachedProvider trace.TracerProvider
	tracerCacheMu  sync.RWMutex

	// Test propagator setup (only needs to happen once)
	setupTestPropagatorOnce sync.Once
)

// tracerKey is the context key type for storing custom tracers
type tracerKey struct{}

// getTracer returns the appropriate tracer for creating spans.
// It uses a smart cache that automatically invalidates when the
// global TracerProvider changes (e.g., in tests).
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
func getTracer(ctx context.Context) trace.Tracer {
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
