# Trace Management System

## Problem & Solution

**Problem**: The original implementation used a global `Tracer` variable that was set once during setup and never updated. This created several issues:
- Tests couldn't easily swap tracers for test isolation
- Provider changes (common in tests via `otel.SetTracerProvider`) were not detected
- No way to inject custom tracers per-request or per-tenant
- Race conditions in parallel tests when swapping providers
- Tight coupling between tracer creation and usage

**Solution**: Smart tracer resolution with automatic provider detection, lazy initialization, and context-based injection support. The system now uses a three-tier resolution strategy:
1. **Context override** - Custom tracers injected via `ContextWithTracer()` (highest priority)
2. **Cached tracer** - Fast path with automatic provider change detection
3. **Provider creation** - Lazy initialization from global `otel.GetTracerProvider()`

## Architecture

### Core Types

- **`GetTracer(ctx)`** - Public API for smart tracer resolution that orchestrates the three-tier lookup strategy. Used internally by all span creation functions (`CreateSpan`, `CreateRootSpan`, `CreateSpanWithOptions`, etc.). Can be called directly for advanced use cases requiring raw tracer access.

- **`ContextWithTracer(ctx, tracer)`** - Public API for context-based tracer injection. Primary use case: parallel test isolation. Stores tracer in context using unexported `tracerKey{}` type.

- **`SetupOTELTester(ctx)`** - Test helper for parallel tests. Returns `(context.Context, *tracetest.SpanRecorder)`. Uses context-based injection for tracer isolation. Sets up propagator once via `sync.Once`.

- **`SetupOTELTesterWithProvider(ctx)`** - Test helper for sequential tests. Returns `(context.Context, *tracetest.SpanRecorder)`. Swaps global provider via `otel.SetTracerProvider()`. WARNING: NOT safe for parallel tests.

- **`tracerKey struct{}`** - Unexported context key type ensuring type safety. Prevents collisions with user code using string keys.

### Design Decisions

**Why smart caching with provider detection**:
- **Performance**: Production apps never swap providers → RWMutex read + pointer comparison (~10-50ns)
- **Test flexibility**: Provider swaps are detected automatically → cache invalidates on change
- **Zero configuration**: Works transparently with `otel.SetTracerProvider()` calls
- Follows Go's "make zero values useful" principle

**Why context-based injection (not context.Context field)**:
- **Type safety**: Unexported `tracerKey struct{}` prevents user code from accidentally overwriting
- **Explicit opt-in**: Production code doesn't pay for context lookup overhead
- **Test isolation**: Parallel tests can use different tracers without global state conflicts
- Follows OpenTelemetry's existing pattern (`trace.ContextWithSpan`)

**Why two test helpers (SetupOTELTester vs SetupOTELTesterWithProvider)**:
- **Different safety guarantees**: Context-based (parallel-safe) vs provider-based (simpler but sequential only)
- **sync.Once optimization**: Parallel helper uses `sync.Once` for propagator setup (efficient for 100+ tests), sequential helper doesn't (flexibility for different configs)
- **Migration path**: Existing tests using `otel.SetTracerProvider()` can use `SetupOTELTesterWithProvider` with minimal changes
- **Clarity**: Explicit function names make safety guarantees obvious at call site

**Why double-check locking pattern**:
- **Fast read path**: RLock for valid cache hits (production hot path)
- **Rare write path**: Lock only when provider changed (test provider swaps)
- **Thread safety**: Prevents cache corruption when multiple goroutines swap providers
- Standard Go pattern for lazy initialization with caching

**Why `sync.Once` only in SetupOTELTester**:
- **Parallel efficiency**: 100+ parallel tests shouldn't redundantly set the same propagator
- **Sequential flexibility**: Sequential tests may want different propagator configurations
- **Idempotent operation**: `otel.SetTextMapPropagator()` is idempotent, so `sync.Once` is optimization not correctness
- Balances performance (parallel tests) with flexibility (sequential tests)

**Why keep deprecated `Tracer` variable**:
- **Backward compatibility**: Existing code using `instrumentation.Tracer.Start()` continues to work
- **Automatic sync**: `getTracer()` updates this variable on cache invalidation
- **Migration path**: Users can migrate gradually using documented deprecation warnings
- Follows semantic versioning (breaking changes only in v2.0)

### Resolution Flow

```
CreateSpan(ctx, "operation") called
         ↓
    GetTracer(ctx)  ← Public API, can be called directly
         ↓
 ┌───────────────────┐
 │ Priority 1        │
 │ Context Override? │ → Yes → Return ctx.Value(tracerKey{})
 └───────────────────┘                    ↓
         No                           DONE (custom tracer)
         ↓
 ┌───────────────────┐
 │ Priority 2        │
 │ Cache Valid?      │ → Yes → Return cachedTracer (RLock)
 │ (RWMutex read)    │                    ↓
 └───────────────────┘                DONE (fast path ~10-50ns)
         No
         ↓
 ┌───────────────────┐
 │ Priority 3        │
 │ Cache Miss/       │
 │ Provider Changed  │
 └───────────────────┘
         ↓
 Lock (write lock)
         ↓
 Double-check (another goroutine might have updated)
         ↓
 currentProvider := otel.GetTracerProvider()
         ↓
 cachedTracer = otel.Tracer(name, version)
 cachedProvider = currentProvider
 Tracer = cachedTracer  // Sync deprecated variable
         ↓
 Unlock
         ↓
 Return cachedTracer
         ↓
     DONE (slow path ~1-10μs, rare)
```

### Provider Change Detection

The cache uses **pointer equality** to detect provider swaps:

```go
currentProvider := otel.GetTracerProvider()  // Get current provider
if cachedProvider == currentProvider {       // Pointer comparison (fast)
    return cachedTracer                      // Cache hit
}
// Cache miss - provider was swapped
```

**Why this works**:
- Each `TracerProvider` is a distinct Go object with unique address
- `otel.SetTracerProvider(newTP)` changes the global provider pointer
- Pointer comparison is O(1) and extremely fast
- No need for version numbers, change notifications, or complex invalidation

**Example provider swap detection**:

```go
// Test setup
tp1 := trace.NewTracerProvider(...)
otel.SetTracerProvider(tp1)

_, span1 := instrumentation.CreateSpan(ctx, "op1")
// → GetTracer() creates tracer, caches (provider=tp1)

// Provider swap (common in tests)
tp2 := trace.NewTracerProvider(...)
otel.SetTracerProvider(tp2)

_, span2 := instrumentation.CreateSpan(ctx, "op2")
// → GetTracer() detects tp2 != tp1, invalidates cache, creates new tracer
```

### Integration Points

**Consumed by**:
- `CreateSpan(ctx, name, kv...)` - Creates child spans using resolved tracer
- `CreateRootSpan(ctx, name, kv...)` - Creates root spans using resolved tracer
- `CreateSpanWithOptions(ctx, name, opts...)` - Type-safe child span creation
- `CreateRootSpanWithOptions(ctx, name, opts...)` - Type-safe root span creation
- `CreateServerSpan`, `CreateClientSpan`, `CreateProducerSpan`, `CreateConsumerSpan` - Convenience functions with pre-configured span kinds
- `GetSpanForContext()` (deprecated) - Backward compatibility wrapper
- **Direct usage**: Advanced users can call `GetTracer(ctx)` directly when they need raw tracer access without SpanLogger integration

**Depends on**:
- `otel.GetTracerProvider()` - Global provider (set by `SetupOTelSDKWithOptions`)
- `otel.SetTracerProvider()` - Provider swap mechanism (tests)
- `otel.SetTextMapPropagator()` - Distributed tracing propagation (test helpers)
- `version.GetInstrumentationName/Version()` - Automatic instrumentation scope versioning

**Events/Hooks**:
- Provider change detection automatically invalidates cache
- Cache updates synchronize deprecated `Tracer` variable for backward compatibility

## GetTracer() Public API

### Overview

`GetTracer(ctx)` is the public API for accessing the smart tracer resolution system. While most application code should use the high-level SpanLogger API (`CreateSpan`, `CreateSpanWithOptions`, etc.), direct tracer access is available for advanced scenarios.

### When to Use GetTracer()

**Use GetTracer() when**:
- Integrating with OpenTelemetry libraries that require a raw `trace.Tracer`
- Building custom instrumentation utilities
- Implementing specialized span creation patterns not covered by the SpanLogger API
- Migrating legacy code that used the deprecated `Tracer` variable

**Don't use GetTracer() for**:
- Regular application tracing (use `CreateSpan`, `CreateSpanWithOptions`, etc.)
- Any scenario where SpanLogger integration is beneficial (structured logging + tracing)
- Simple span creation with attributes/span kinds (use convenience functions)

### Trade-offs

**What you gain**:
- Full control over OpenTelemetry span creation
- Access to all `trace.Tracer.Start()` options
- Flexibility for custom instrumentation patterns
- Integration with third-party OTel libraries

**What you lose**:
- SpanLogger integration (no automatic log/trace correlation)
- Compile-time safety (raw `trace.Span` doesn't enforce `End()` calls)
- Structured logging with trace context
- Logger enrichment with operation context

### Examples

#### Basic Tracer Access

```go
import (
    "go.opentelemetry.io/otel/trace"
    "github.com/weka/go-weka-observability/instrumentation"
)

func customInstrumentation(ctx context.Context) {
    // Get the smart-resolved tracer
    tracer := instrumentation.GetTracer(ctx)

    // Use it directly for OpenTelemetry span creation
    ctx, span := tracer.Start(ctx, "custom-operation",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("custom.key", "value"),
        ),
    )
    defer span.End()

    // Manual attribute setting
    span.SetAttributes(attribute.Int("result", 42))

    // Note: No automatic logging or trace ID injection
}
```

#### Integration with OTel Libraries

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "github.com/weka/go-weka-observability/instrumentation"
)

func setupHTTPClient(ctx context.Context) *http.Client {
    // Get tracer for OTel library integration
    tracer := instrumentation.GetTracer(ctx)

    // Use with third-party OTel instrumentation
    transport := otelhttp.NewTransport(
        http.DefaultTransport,
        otelhttp.WithTracerProvider(trace.TracerProvider{
            Tracer: func(name string, opts ...trace.TracerOption) trace.Tracer {
                return tracer
            },
        }),
    )

    return &http.Client{Transport: transport}
}
```

#### Custom Span Context Manipulation

```go
func advancedSpanManagement(ctx context.Context) {
    tracer := instrumentation.GetTracer(ctx)

    // Create span with custom trace ID (advanced use case)
    spanContext := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    traceID,
        SpanID:     spanID,
        TraceFlags: trace.FlagsSampled,
    })

    ctx = trace.ContextWithSpanContext(ctx, spanContext)
    ctx, span := tracer.Start(ctx, "custom-trace-operation")
    defer span.End()

    // Complex span manipulation...
}
```

### Comparison: GetTracer() vs SpanLogger API

```go
// Using GetTracer() directly
func withRawTracer(ctx context.Context) {
    tracer := instrumentation.GetTracer(ctx)
    ctx, span := tracer.Start(ctx, "database-query",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            attribute.String("db.system", "postgresql"),
        ),
    )
    defer span.End()

    // Manual logging (no trace correlation)
    log.Info("Querying database")
    span.SetAttributes(attribute.Int("rows", 100))
}

// Using SpanLogger API (recommended for most cases)
func withSpanLogger(ctx context.Context) {
    // Create client span
    ctx, logger := instrumentation.CreateClientSpan(ctx, "database-query")
    defer logger.End()

    // Add attributes to BOTH logger and span using WithValues
    ctx, logger = logger.WithValues(
        "db.system", "postgresql",
        "rows", 100,
    )

    // Unified logging with automatic trace correlation
    logger.Info("Querying database")
    // ^ Log automatically includes trace_id, span_id, db.system, rows
}
```

### Migration from Deprecated Tracer Variable

```go
// OLD (deprecated)
ctx, span := instrumentation.Tracer.Start(ctx, "operation")
defer span.End()

// NEW (advanced cases only)
tracer := instrumentation.GetTracer(ctx)
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// RECOMMENDED (for most application code)
ctx, logger := instrumentation.CreateSpan(ctx, "operation")
defer logger.End()
```

## Usage

### Basic Usage (Production)

```go
package main

import (
    "context"
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

func main() {
    ctx := context.Background()

    // Setup logging (required for SpanLogger)
    logr := logger.CreateLogger()
    ctx = logger.ContextWithLogr(ctx, logr)

    // Setup OpenTelemetry (sets global provider)
    shutdown, err := instrumentation.SetupOTelSDKWithOptions(
        ctx, "my-service", "v1.0.0", logr,
    )
    if err != nil {
        panic(err)
    }
    defer shutdown(ctx)

    // Create spans - getTracer(ctx) resolves automatically
    processRequest(ctx)
}

func processRequest(ctx context.Context) {
    // CreateSpan uses getTracer(ctx) internally
    // → Checks context (no override)
    // → Checks cache (hit: fast path)
    // → Returns cached tracer
    ctx, logger := instrumentation.CreateSpan(ctx, "process_request")
    defer logger.End()

    logger.Info("Processing started")
    queryDatabase(ctx)  // Child span
}

func queryDatabase(ctx context.Context) {
    ctx, logger := instrumentation.CreateSpan(ctx, "query_database")
    defer logger.End()

    logger.Info("Querying database")
    // Nested span under process_request
}
```

### Advanced Scenarios

#### Parallel Test Isolation

```go
package mypackage_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/require"
    "github.com/weka/go-weka-observability/instrumentation"
)

func TestFeatureA(t *testing.T) {
    t.Parallel()  // ✅ Safe - uses context-based tracer injection

    ctx := context.Background()
    ctx, recorder := instrumentation.SetupOTELTester(ctx)
    defer recorder.Shutdown(context.Background())

    // All CreateSpan calls use tracer from context
    // getTracer(ctx) → Priority 1: ctx.Value(tracerKey{})
    ctx, logger := instrumentation.CreateSpan(ctx, "feature_a")
    defer logger.End()

    logger.Info("Testing feature A")

    // Verify spans
    spans := recorder.Ended()
    require.Len(t, spans, 1)
    require.Equal(t, "feature_a", spans[0].Name())
}

func TestFeatureB(t *testing.T) {
    t.Parallel()  // ✅ Safe - isolated from TestFeatureA

    ctx := context.Background()
    ctx, recorder := instrumentation.SetupOTELTester(ctx)
    defer recorder.Shutdown(context.Background())

    ctx, logger := instrumentation.CreateSpan(ctx, "feature_b")
    defer logger.End()

    logger.Info("Testing feature B")

    spans := recorder.Ended()
    require.Len(t, spans, 1)
    require.Equal(t, "feature_b", spans[0].Name())
}
```

#### Sequential Tests with Provider Swap

```go
func TestDistributedTracing(t *testing.T) {
    // ⚠️ NO t.Parallel() - swaps global provider

    ctx := context.Background()
    ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
    defer recorder.Shutdown(context.Background())

    // Provider was swapped globally
    // getTracer(ctx) detects change:
    // → cachedProvider != otel.GetTracerProvider()
    // → Creates new tracer from swapped provider

    ctx, logger := instrumentation.CreateSpan(ctx, "distributed_trace")
    defer logger.End()

    // Test propagation via HTTP headers, etc.
}
```

#### Custom Tracer for Multi-Tenant Apps

```go
func handleTenantRequest(ctx context.Context, tenantID string) {
    // Create tenant-specific tracer
    recorder := tracetest.NewSpanRecorder()
    tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
    tenantTracer := tp.Tracer(fmt.Sprintf("tenant-%s", tenantID))

    // Inject via context (overrides global provider)
    ctx = instrumentation.ContextWithTracer(ctx, tenantTracer)

    // All CreateSpan calls in this request use tenant tracer
    ctx, logger := instrumentation.CreateSpan(ctx, "handle_tenant_request")
    defer logger.End()

    logger.Info("Processing tenant request", "tenant_id", tenantID)

    // Spans go to tenant-specific recorder
}
```

#### Provider Detection in Tests

```go
func TestProviderSwapDetection(t *testing.T) {
    ctx := context.Background()

    // Provider 1
    recorder1 := tracetest.NewSpanRecorder()
    tp1 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder1))
    otel.SetTracerProvider(tp1)

    _, span1 := instrumentation.CreateSpan(ctx, "op1")
    span1.End()

    require.Len(t, recorder1.Ended(), 1)  // Span went to recorder1

    // Provider swap
    recorder2 := tracetest.NewSpanRecorder()
    tp2 := trace.NewTracerProvider(trace.WithSpanProcessor(recorder2))
    otel.SetTracerProvider(tp2)

    // getTracer() automatically detects provider change
    _, span2 := instrumentation.CreateSpan(ctx, "op2")
    span2.End()

    require.Len(t, recorder2.Ended(), 1)  // Span went to recorder2
    require.Len(t, recorder1.Ended(), 1)  // recorder1 unchanged
}
```

## Testing Strategy

### Unit Tests

**tracer_test.go** covers core resolution logic:

- **`TestGetTracerProviderDetection`** - Verifies automatic provider change detection
  - Creates two providers with different recorders
  - Swaps provider via `otel.SetTracerProvider()`
  - Verifies spans route to correct recorder after swap
  - **Coverage**: Provider pointer comparison and cache invalidation

- **`TestContextOverride`** - Verifies context-based tracer injection takes priority
  - Sets up global provider (should be ignored)
  - Injects custom tracer via `ContextWithTracer()`
  - Verifies spans use context tracer, not global provider
  - **Coverage**: Priority 1 (context override) in resolution flow

- **`TestProviderSwapPattern`** - Validates parallel-safe test helper
  - Uses `SetupOTELTester()` with `t.Parallel()`
  - Creates spans and verifies they're recorded
  - **Coverage**: Context-based injection for parallel test safety

- **`TestConcurrentAccess`** - Race condition detection with -race flag
  - Spawns 100 goroutines calling `CreateSpan()` concurrently
  - Validates all spans created successfully
  - **Coverage**: Thread safety of `getTracer()` double-check locking

- **`TestCacheHitPerformance`** - Fast path validation
  - Creates multiple spans without provider changes
  - Verifies all spans use same cached tracer
  - **Coverage**: RWMutex read performance optimization

### Test Strategy

**Unit test coverage**: 100% of `getTracer()` logic
- Context override path (Priority 1)
- Cache hit path (Priority 2 - fast)
- Cache miss path (Priority 3 - provider change)
- Thread safety (double-check locking)

**Integration test coverage**:
- **logspan_test.go** - Testify suite using `SetupOTELTesterWithProvider`
- **Parallel test validation** - Multiple tests with `t.Parallel()` using `SetupOTELTester`

**Race detector coverage**: All tests pass with `go test -race`
- Concurrent access to cache
- Provider swap detection during concurrent span creation
- Context-based injection in parallel tests

### Test Helpers Comparison

| Feature | SetupOTELTester | SetupOTELTesterWithProvider |
|---------|----------------|---------------------------|
| Parallel safe | ✅ Yes (context-based) | ❌ No (global provider) |
| Use with t.Parallel() | ✅ Recommended | ❌ Forbidden |
| Propagator setup | sync.Once (efficient) | Every call (flexible) |
| Setup complexity | Medium | Simple |
| Best for | 100+ parallel tests | Sequential integration tests |
| Provider detection | N/A (context overrides) | ✅ Automatic |

## Performance Characteristics

### Production Performance

**Typical case (provider never changes)**:
```
getTracer(ctx) performance:
1. ctx.Value() lookup:        ~20-30ns  (context check)
2. RLock:                      ~10-20ns  (read lock)
3. Pointer comparison:         ~5ns      (provider check)
4. RUnlock:                    ~10-20ns  (unlock)
   ─────────────────────────────────────
   Total per CreateSpan call:  ~45-75ns
```

**Edge case (provider swap in tests)**:
```
getTracer(ctx) performance:
1. Context check:              ~20-30ns  (miss)
2. RLock:                      ~10-20ns  (cache check)
3. RUnlock:                    ~10-20ns  (invalid)
4. Lock (write):               ~50-100ns (exclusive lock)
5. Provider change check:      ~5ns      (double-check)
6. otel.Tracer() creation:     ~1-10μs   (new tracer)
7. Unlock:                     ~50-100ns (release)
   ─────────────────────────────────────
   Total on provider swap:     ~1-10μs   (rare event)
```

### Memory Overhead

**Cache storage**:
```go
var (
    cachedTracer   trace.Tracer         // ~24 bytes (interface)
    cachedProvider trace.TracerProvider // ~24 bytes (interface)
    tracerCacheMu  sync.RWMutex        // ~24 bytes (sync primitives)
)
Total: ~72 bytes static memory
```

**Per-context overhead**: 0 bytes (unless using `ContextWithTracer`)

**With context injection**: +48 bytes per context (key + tracer interface)

### Scalability

**Concurrency**: Unlimited concurrent `getTracer()` calls
- Read-heavy workload (production): No lock contention
- Write-rare workload (test provider swaps): Brief lock contention

**Test parallelism**: Unlimited parallel tests with `SetupOTELTester`
- Each test has isolated context tracer
- sync.Once for propagator setup (one-time cost)
- Zero cross-test interference

## Migration from Old Code

### Application Code (using global Tracer)

**Before**:
```go
ctx, span := instrumentation.Tracer.Start(ctx, "operation")
defer span.End()

span.SetAttributes(attribute.String("key", "value"))
```

**After**:
```go
ctx, logger := instrumentation.CreateSpan(ctx, "operation", "key", "value")
defer logger.End()

logger.Info("Operation started")  // Automatic trace correlation
```

**Benefits**:
- Unified logging + tracing
- Automatic trace ID injection in logs
- No more manual attribute setting for logs

### Test Code (manual provider setup)

**Before**:
```go
func TestFeature(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
    otel.SetTracerProvider(tp)
    defer tp.Shutdown(context.Background())

    // Test code...
}
```

**After (parallel-safe)**:
```go
func TestFeature(t *testing.T) {
    t.Parallel()  // ✅ Now safe!

    ctx := context.Background()
    ctx, recorder := instrumentation.SetupOTELTester(ctx)
    defer recorder.Shutdown(context.Background())

    // Test code using ctx...
}
```

**After (sequential, simpler)**:
```go
func TestFeature(t *testing.T) {
    ctx := context.Background()
    ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
    defer recorder.Shutdown(context.Background())

    // Test code...
}
```

## Future Considerations

### Known Limitations

1. **Propagator is still global**: `SetupOTELTester` uses `sync.Once` for propagator, so all parallel tests share the same propagator configuration. This is acceptable because:
   - OpenTelemetry doesn't provide context-based propagators
   - Most tests don't need different propagator configs
   - `SetupOTELTesterWithProvider` allows custom config for sequential tests

2. **Deprecated `Tracer` variable**: Kept for backward compatibility but not recommended. Will be removed in v2.0. Users should migrate to `CreateSpan` API.

3. **Context value overhead**: Using `ContextWithTracer` adds ~48 bytes per context. Negligible for most use cases but could matter in extremely memory-constrained environments with millions of contexts.

### Potential Extensions

1. **Context-based propagator injection**: If OpenTelemetry adds context-based propagators in the future, we could extend `ContextWithTracer` to also inject propagators for complete test isolation.

2. **Tracer pools**: For multi-tenant applications with many tenants, could add a tracer pool to avoid creating a new tracer per request. Would trade memory (pool storage) for allocation overhead.

3. **Metrics for provider swaps**: Could expose metrics on how often provider swaps occur (useful for identifying test setup issues).

4. **Debug mode**: Could add a debug mode that logs tracer resolution decisions (`TRACE_DEBUG=1`) to help diagnose setup issues.

### Related Features

- **SpanLogger API** (docs/spanlogger-api.md) - Uses `getTracer()` internally
- **OpenTelemetry Configuration** (docs/instrumentation-configuration-api.md) - Provider setup
- **Logger Integration** (docs/logger-configuration-api.md) - Context-based logger storage (similar pattern)

## References

- **OpenTelemetry Go SDK**: https://pkg.go.dev/go.opentelemetry.io/otel
- **TracerProvider pattern**: https://opentelemetry.io/docs/specs/otel/trace/api/#tracerprovider
- **Context propagation**: https://opentelemetry.io/docs/specs/otel/context/
- **Testing with tracetest**: https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace/tracetest
- **Double-check locking**: https://en.wikipedia.org/wiki/Double-checked_locking

## Design Patterns Used

- **Lazy initialization**: Tracer created on first `CreateSpan()` call, not during setup
- **Caching with invalidation**: Provider pointer comparison for automatic cache invalidation
- **Double-check locking**: Thread-safe optimization for read-heavy workloads
- **Strategy pattern**: Three-tier resolution (context → cache → provider)
- **Context-based dependency injection**: `ContextWithTracer()` for test isolation
- **Sync.Once**: One-time propagator setup for parallel test efficiency
