# Instrumentation Configuration Architecture

## Overview

The `instrumentation` package provides OpenTelemetry-based distributed tracing with flexible configuration and environment-based overrides. It follows the same configuration pattern as the `logger` package, making it intuitive and consistent across the observability toolkit.

**Key Features:**
- Type-safe configuration with environment variable overrides
- Multiple API patterns (config-based and functional options)
- Automatic trace context propagation
- Integration with logr.Logger for unified observability
- Production-ready with graceful degradation (no endpoint = no export)
- Combined logging and tracing via SpanLogger API (`CreateLogSpan`, `CurrentSpanLogger`, `CreateRootLogSpan`)

**📖 See Also:**
- **[SpanLogger API Documentation](spanlogger-api.md)** - Complete guide to span creation and lifecycle management
- **[Examples](../examples/)** - Runnable examples demonstrating span usage patterns

---

## Quick Start

### Basic Setup (Environment Variable Only)

```go
import (
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

func main() {
    ctx := context.Background()

    // Create logger with explicit options (overrideable via LOG_* env vars)
    logr := logger.CreateLogger(
        logger.WithConsoleSink(),
        logger.WithInfoLevel(),
    )
    ctx = logger.ContextWithLogr(ctx, logr)

    // Setup OpenTelemetry (uses OTEL_EXPORTER_OTLP_ENDPOINT env var)
    shutdown, err := instrumentation.SetupOTelSDKWithOptions(
        ctx, "my-service", "v1.0.0", logr,
    )
    if err != nil {
        panic(err)
    }
    defer shutdown(ctx)

    // Create a traced operation with automatic logging
    ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "operation", "user", "alice")
    defer spanLogger.End() // Required!

    spanLogger.Info("Operation started")
}
```

**Environment variable:**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
```

**📖 For more span API patterns:** See [SpanLogger API Documentation](spanlogger-api.md) for `CurrentSpanLogger` and `CreateRootLogSpan` usage.

### With Fallback Endpoint (Env Always Takes Precedence)

```go
// Set a fallback endpoint to use when OTEL_EXPORTER_OTLP_ENDPOINT is not set
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "my-service", "v1.0.0", logr,
    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
    instrumentation.WithResourceAttributes("environment", "production", "region", "us-west"),
)
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable always takes precedence if set
```

### With Explicit Config

```go
// Full control over configuration
config := instrumentation.OTelConfig{
    Endpoint: "http://otel-collector:4317",  // DEFAULT value
}
config = instrumentation.NewOTelConfigFromEnv(config)  // Env can override

shutdown, err := instrumentation.SetupOTelSDKFrom(
    ctx, "my-service", "v1.0.0", logr, config,
    "environment", "production",  // Resource attributes
)
```

---

## Architecture

### Design Principles

1. **Default + Override Pattern** - Same as logger package (defaults can be overridden by env vars)
2. **Environment-Driven** - Support 12-factor app configuration via `OTEL_*` env vars
3. **Graceful Degradation** - No endpoint = no export (application continues normally)
4. **Unified Observability** - Combined logging and tracing via `SpanLogger`
5. **Context Propagation** - Automatic trace context through `context.Context`

### Configuration Flow

```
Application
    ↓
OTelConfig (with defaults)
    ↓
NewOTelConfigFromEnv() (env overrides)
    ↓
SetupOTelSDKInternal() (initialization)
    ↓
    ├─→ TracerProvider (with exporter)
    ├─→ TextMapPropagator (for context propagation)
    └─→ Tracer Cache (with provider detection)
```

### Trace + Log Flow

```
Application Code
    ↓
GetLogSpan(ctx, "operation")
    ↓
    ├─→ Creates OpenTelemetry Span
    └─→ Creates logr.Logger with trace/span IDs
    ↓
SpanLogger (combined logger + span)
    ↓
    ├─→ spanLogger.Info() → logs to logger + adds span event
    ├─→ spanLogger.Error() → logs error + records span error
    └─→ defer end() → closes span, logs completion
```

---

## Configuration API

### Configuration Priority

All configuration APIs follow this priority order (highest to lowest):

1. **Environment Variables** - `OTEL_EXPORTER_OTLP_ENDPOINT`
2. **Functional Options or Config** - API-provided defaults
3. **Built-in Defaults** - Empty endpoint (no export)

### API 1: Config-Based (Explicit)

**Pattern:** Mirrors `logger.CreateLoggerFrom` - explicit config with env overrides.

```go
// Step 1: Create config with your defaults
config := instrumentation.OTelConfig{
    Endpoint: "http://default-collector:4317",
    ResourceAttributes: []any{"app", "myapp"},
}

// Step 2: Allow environment variable overrides
config = instrumentation.NewOTelConfigFromEnv(config)

// Step 3: Initialize SDK
shutdown, err := instrumentation.SetupOTelSDKFrom(
    ctx, "my-service", "v1.0.0", logger, config,
    "additional", "attributes",  // Additional resource attributes
)
```

**When to use:**
- You need full control over configuration
- You want to build config from multiple sources
- You're migrating from config files

### API 2: Functional Options (Recommended)

**Pattern:** Mirrors `logger.CreateLogger` - options set fallback values, env always takes precedence.

```go
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "my-service", "v1.0.0", logger,
    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
    instrumentation.WithResourceAttributes("environment", "production", "region", "us-west"),
)
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable always takes precedence if set,
// regardless of whether you use WithDefaultOTLPEndpoint or not
```

**When to use:**
- Clean, fluent API (recommended for most use cases)
- You want fallback values that can be overridden by env vars
- You're starting a new project

### API 3: Legacy (Deprecated)

**Pattern:** Original API with only env var support.

```go
// ⚠️ Deprecated - no endpoint configuration via API
shutdown, err := instrumentation.SetupOTelSDK(ctx, "service", "v1", logger, "key", "value")
// Only OTEL_EXPORTER_OTLP_ENDPOINT environment variable works
```

**Migration:**
- Replace with `SetupOTelSDKWithOptions` for functional options
- Replace with `SetupOTelSDKFrom` for config-based approach

---

## Configuration Options

### OTelConfig Structure

```go
type OTelConfig struct {
    // Endpoint is the OTLP exporter endpoint (e.g., "http://localhost:4317")
    // Empty string means no traces will be exported
    Endpoint string `envconfig:"EXPORTER_OTLP_ENDPOINT"`

    // ResourceAttributes are additional key-value pairs attached to all spans
    ResourceAttributes []any
}
```

### Functional Options

#### WithDefaultOTLPEndpoint

Sets the OTLP endpoint to use when `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable is not set.

```go
instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317")
```

**Example:**
```go
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "my-service", "v1.0.0", logger,
    instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
)

// OTEL_EXPORTER_OTLP_ENDPOINT always takes precedence if set:
// export OTEL_EXPORTER_OTLP_ENDPOINT=http://prod-collector:4317
```

#### WithResourceAttributes

Sets resource attributes attached to all spans (metadata about your service).

```go
instrumentation.WithResourceAttributes("key1", "value1", "key2", "value2")
```

**Common attributes:**
- `environment`: `"production"`, `"staging"`, `"development"`
- `region`: `"us-west"`, `"eu-central"`, `"ap-southeast"`
- `cluster`: `"cluster-a"`, `"cluster-b"`
- `version`: `"v2.1.0"`

**Example:**
```go
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "my-service", "v1.0.0", logger,
    instrumentation.WithResourceAttributes(
        "environment", "production",
        "region", "us-west-2",
        "cluster", "main-cluster",
    ),
)
```

---

## Environment Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | OTLP collector endpoint (e.g., `http://localhost:4317`) |

**Example:**
```bash
# Development
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

# Production
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.prod.example.com:4317
```

### Environment Override Behavior

```go
// Code sets fallback value
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "service", "v1", logger,
    instrumentation.WithDefaultOTLPEndpoint("http://dev-collector:4317"),
)

// Environment variable ALWAYS takes precedence if set
// export OTEL_EXPORTER_OTLP_ENDPOINT=http://prod-collector:4317
// Result: Uses http://prod-collector:4317 (env var wins)

// If env var is NOT set, uses http://dev-collector:4317 (fallback)
```

### No Endpoint = No Export

If no endpoint is configured (neither in code nor env), tracing setup succeeds but traces are not exported:

```go
// No endpoint configured
shutdown, err := instrumentation.SetupOTelSDKWithOptions(ctx, "service", "v1", logger)
// err == nil, but traces won't be exported (graceful degradation)
```

---

## Combined Logging and Tracing

### SpanLogger API - Unified Observability

The SpanLogger API provides three functions for different span ownership patterns:

**1. CreateLogSpan - Create Owned Spans (Most Common)**
```go
ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "operation-name", "user_id", 123)
defer spanLogger.End() // Required!

spanLogger.Info("Processing request")
spanLogger.Error(err, "Failed to process", "retry_count", 3)
```

**2. CurrentSpanLogger - Borrow Current Span (Helper Functions)**
```go
// In helper functions that just need to log
view := instrumentation.CurrentSpanLogger(ctx)
view.Info("Helper function working")
// No End() call - compile-time safety!
```

**3. CreateRootLogSpan - Independent Traces (Background Jobs)**
```go
ctx, spanLogger := instrumentation.CreateRootLogSpan(ctx, "background-job", "job_id", "abc")
defer spanLogger.End() // Required!

spanLogger.Info("Background job with new trace ID")
```

**IMPORTANT:** A logger MUST be stored in context before creating spans:

```go
// REQUIRED: Store logger in context first
logr := logger.CreateLogger(logger.WithInfoLevel())
ctx = logger.ContextWithLogr(ctx, logr)

// Now span creation functions can retrieve it
ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "operation")
```

If no logger is in context, a default logger will be created automatically (but you lose control over logger configuration).

**📖 Complete Documentation:**
- **[SpanLogger API Guide](spanlogger-api.md)** - Architecture, design decisions, migration guide
- **[Examples](../examples/)** - Comprehensive runnable examples

**🔄 Migration Note:**
The old `GetLogSpan` API is deprecated. See the [migration guide](spanlogger-api.md#migration-from-getlogspan) for upgrade instructions.

### SpanLogger Methods

#### Info, Debug, Warn

```go
spanLogger.Info("message", "key", "value")
spanLogger.Debug("debug message", "details", data)
spanLogger.Warn("warning message", "reason", "timeout")
```

**Effect:**
- Logs to logger with trace context
- Adds event to OpenTelemetry span

#### Error

```go
if err != nil {
    spanLogger.Error(err, "operation failed", "input", userInput)
}
```

**Effect:**
- Logs error with trace context
- Records error in OpenTelemetry span

#### SetAttributes

```go
spanLogger.SetAttributes(
    attribute.String("custom_field", "value"),
    attribute.Int("count", 42),
)
```

**Effect:**
- Adds attributes to OpenTelemetry span only (not logged)

### Reusing Parent Span (Helper Functions)

**Use case:** Helper functions that should log under the parent's span without creating a new child span.

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "processOrder")
    defer end()

    logger.Info("Processing order", "order_id", orderID)

    // Helper logs under same span (no new span created)
    validateData(ctx, orderID)

    return nil
}

func validateData(ctx context.Context, orderID string) {
    // Empty string ("") reuses parent span
    _, logger, _ := instrumentation.GetLogSpan(ctx, "")
    // DO NOT call end() - no new span created!

    logger.Info("Validating data", "order_id", orderID)
    // Logs include parent's trace_id and span_id
}
```

**IMPORTANT:**
- Empty string (`""`) reuses the current span from context (doesn't create a new one)
- Calling `end()` is safe (no-op) but not recommended for code clarity
- Cannot pass `keysAndValues` when name is empty (will panic)
- Use this pattern for helper functions that don't need their own span

---

## Usage Patterns

### Basic Operation Tracing

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "processOrder")
    defer end()

    logger.Info("Processing order", "order_id", orderID)

    // Your business logic
    if err := validateOrder(ctx, orderID); err != nil {
        logger.Error(err, "Order validation failed")
        return err
    }

    logger.Info("Order processed successfully")
    return nil
}
```

### Nested Operations

```go
func processOrder(ctx context.Context, orderID string) error {
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "processOrder")
    defer end()

    logger.Info("Processing order", "order_id", orderID)

    // Nested operation creates child span
    if err := chargePayment(ctx, orderID); err != nil {
        return err
    }

    if err := shipOrder(ctx, orderID); err != nil {
        return err
    }

    return nil
}

func chargePayment(ctx context.Context, orderID string) error {
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "chargePayment")
    defer end()

    logger.Info("Charging payment", "order_id", orderID)
    // Payment logic
    return nil
}
```

**Trace hierarchy:**
```
processOrder (parent span)
├── chargePayment (child span)
└── shipOrder (child span)
```

### HTTP Client Tracing

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

func callExternalAPI(ctx context.Context) error {
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "callExternalAPI")
    defer end()

    // Wrap HTTP client with otelhttp for automatic trace propagation
    client := &http.Client{
        Transport: otelhttp.NewTransport(http.DefaultTransport),
    }

    req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com/data", nil)
    resp, err := client.Do(req)
    if err != nil {
        logger.Error(err, "API call failed")
        return err
    }
    defer resp.Body.Close()

    logger.Info("API call succeeded", "status", resp.StatusCode)
    return nil
}
```

### HTTP Server Tracing

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

func main() {
    // Initialize instrumentation
    shutdown, _ := instrumentation.SetupOTelSDKWithOptions(ctx, "api-server", "v1", logger)
    defer shutdown(ctx)

    // Wrap HTTP handler with otelhttp for automatic trace extraction
    mux := http.NewServeMux()
    mux.HandleFunc("/api/orders", handleOrders)

    handler := otelhttp.NewHandler(mux, "api-server")
    http.ListenAndServe(":8080", handler)
}

func handleOrders(w http.ResponseWriter, r *http.Request) {
    // Context already has trace from otelhttp middleware
    ctx, logger, end := instrumentation.GetLogSpan(r.Context(), "handleOrders")
    defer end()

    logger.Info("Handling order request", "method", r.Method)
    // Your handler logic
}
```

---

## Testing

### Testing Pattern 1: Context Override (Recommended)

**Best for**: Parallel tests requiring complete isolation without affecting global state.

```go
import (
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestProcessOrder(t *testing.T) {
    t.Parallel()  // ✅ Safe parallel execution

    ctx := context.Background()

    // Create test tracer with span recorder
    recorder := tracetest.NewSpanRecorder()
    tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
    tracer := tp.Tracer("test-tracer")
    defer tp.Shutdown(ctx)

    // Inject via context (no global state mutation)
    ctx = instrumentation.ContextWithTracer(ctx, tracer)

    // Store logger in context
    logr := logger.CreateLogger()
    ctx = logger.ContextWithLogr(ctx, logr)

    // Test your traced functions - all spans use injected tracer
    err := processOrder(ctx, "order-123")
    assert.NoError(t, err)

    // Verify spans were created
    spans := recorder.Ended()
    require.Len(t, spans, 1)
    assert.Equal(t, "processOrder", spans[0].Name())
}
```

**Advantages:**
- ✅ Zero global state mutation
- ✅ Perfect test isolation
- ✅ Can run unlimited parallel tests
- ✅ Each test has independent tracer
- ✅ No cleanup needed

### Testing Pattern 2: Provider Swap

**Best for**: Testing library behavior with different providers.

```go
func TestWithProviderSwap(t *testing.T) {
    t.Parallel()  // ✅ Also safe!

    ctx := context.Background()

    // Create test provider
    recorder := tracetest.NewSpanRecorder()
    tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
    defer tp.Shutdown(ctx)

    // Swap global provider
    otel.SetTracerProvider(tp)

    // Store logger in context
    logr := logger.CreateLogger()
    ctx = logger.ContextWithLogr(ctx, logr)

    // getTracer() automatically detects provider change
    err := processOrder(ctx, "order-123")
    assert.NoError(t, err)

    // Verify spans
    spans := recorder.Ended()
    require.Len(t, spans, 1)
}
```

**How it works:**
1. Test swaps global provider with `otel.SetTracerProvider()`
2. Next span creation detects provider change (pointer comparison)
3. Cache invalidated, new tracer created from new provider
4. All subsequent calls use new tracer

**Advantages:**
- ✅ Tests library's provider integration
- ✅ Matches OpenTelemetry best practices
- ✅ No context passing needed for tracer
- ✅ Automatic cache invalidation

### Testing with Custom Endpoint

```go
func TestWithCollector(t *testing.T) {
    ctx := context.Background()
    logr := logger.CreateLogger()

    // Override endpoint for testing
    t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://test-collector:4317")

    shutdown, err := instrumentation.SetupOTelSDKWithOptions(
        ctx, "test-service", "v1.0.0", logr,
    )
    require.NoError(t, err)
    defer shutdown(ctx)

    // Your tests
}
```

### Testing Pattern 3: Helper Functions (Simplest)

**Best for**: Quick test setup with minimal boilerplate.

The `instrumentation/oteltest` package provides two helper functions for common testing scenarios:

#### oteltest.SetupTester (Parallel-Safe)

For most tests, use `oteltest.SetupTester()` which is safe for parallel execution:

```go
import "github.com/weka/go-weka-observability/instrumentation/oteltest"

func TestMyFeature(t *testing.T) {
    t.Parallel()  // Safe for parallel execution

    ctx := context.Background()
    ctx, recorder := oteltest.SetupTester(ctx)
    defer recorder.Shutdown(context.Background())

    // Store logger in context
    logr := logger.CreateLogger()
    ctx = logger.ContextWithLogr(ctx, logr)

    // Your test code using CreateLogSpan, CreateRootLogSpan, etc.
    ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "operation")
    defer spanLogger.End()

    spanLogger.Info("Processing request")

    // Assertions
    spans := recorder.Ended()
    require.Len(t, spans, 1)
    assert.Equal(t, "operation", spans[0].Name())
}
```

**Advantages:**
- Minimal boilerplate (2 lines setup)
- Safe for parallel tests (context-based tracer isolation)
- Sets up propagator for production code that relies on trace propagation
- Returns context with cached tracer and span recorder
- Perfect for unit tests

**Note:**
This function sets the global `TextMapPropagator` (required for propagation to work), but this is safe for parallel tests because:
- The propagator setup uses `sync.Once` (only happens once per process, efficient for 100+ tests)
- Tracer isolation is achieved via context (no cross-test interference)
- OpenTelemetry propagators are only available via global state (no alternative)

#### oteltest.SetupTesterWithProvider (Sequential Tests Only)

For integration tests that need TextMapPropagator (distributed tracing):

```go
import "github.com/weka/go-weka-observability/instrumentation/oteltest"

func TestDistributedTracing(t *testing.T) {
    // NO t.Parallel() - not safe with provider swap

    ctx := context.Background()
    ctx, recorder := oteltest.SetupTesterWithProvider(ctx)
    defer recorder.Shutdown(context.Background())

    // Store logger in context
    logr := logger.CreateLogger()
    ctx = logger.ContextWithLogr(ctx, logr)

    // Test code that uses propagation (HTTP headers, etc.)
    // CreateLogSpan will use the swapped provider automatically
    // Propagator is already set up for distributed tracing

    // Assertions
    spans := recorder.Ended()
    require.Len(t, spans, 1)
}
```

**When to use:**
- Integration tests requiring TextMapPropagator
- Tests verifying distributed tracing behavior
- Tests that need provider swap behavior

**Warning:**
This function swaps the global TracerProvider and TextMapPropagator which are shared across all goroutines. While the global state itself is thread-safe, parallel tests will interfere with each other by overriding each other's providers.

---

## Migration Guide

### From SetupOTelSDK (Deprecated)

**Before:**
```go
shutdown, err := instrumentation.SetupOTelSDK(
    ctx, "service", "v1", logger,
    "key1", "value1",
    "key2", "value2",
)
```

**After (Functional Options):**
```go
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "service", "v1", logger,
    instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
    instrumentation.WithResourceAttributes("key1", "value1", "key2", "value2"),
)
```

**After (Config-Based):**
```go
config := instrumentation.NewDefaultOTelConfigWithEnvOverrides()
shutdown, err := instrumentation.SetupOTelSDKFrom(
    ctx, "service", "v1", logger, config,
    "key1", "value1",
    "key2", "value2",
)
```

---

## Best Practices

### 1. Always Defer Shutdown

```go
shutdown, err := instrumentation.SetupOTelSDKWithOptions(ctx, "service", "v1", logger)
if err != nil {
    return err
}
defer shutdown(ctx)  // ✅ Ensures traces are flushed
```

### 2. Use SpanLogger API for All Operations

```go
// ✅ Good - unified logging and tracing with CreateSpan
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
defer logger.End()
logger.Info("Processing")

// ❌ Bad - separate logging and tracing (low-level OTel API)
span := trace.SpanFromContext(ctx)
logger := logr.FromContext(ctx)
```

**📖 See:** [SpanLogger API Documentation](spanlogger-api.md) for `CurrentSpanLogger` and `CreateRootLogSpan` patterns.

### 3. Use Descriptive Span Names

```go
// ✅ Good - specific operation names
instrumentation.CreateLogSpan(ctx, "database.query.users")
instrumentation.CreateLogSpan(ctx, "payment.charge")
instrumentation.CreateLogSpan(ctx, "email.send")

// ❌ Bad - generic names
instrumentation.CreateLogSpan(ctx, "process")
instrumentation.CreateLogSpan(ctx, "handle")
```

### 4. Add Meaningful Attributes

```go
// ✅ Good - business context
spanLogger.Info("Order created",
    "order_id", order.ID,
    "customer_id", order.CustomerID,
    "total_amount", order.Total,
)

// ❌ Bad - technical noise only
spanLogger.Info("Database insert completed")
```

### 5. Set Fallback Values (Env Always Wins)

```go
// ✅ Good - fallback for dev, env var for prod
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "service", "v1", logger,
    instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
)
// Production: export OTEL_EXPORTER_OTLP_ENDPOINT=https://prod-collector:4317
// The env var will always be used if set, regardless of WithDefaultOTLPEndpoint
```

---

## Troubleshooting

### Traces Not Appearing

**Problem:** No traces in your collector/backend.

**Solutions:**
1. Check endpoint configuration:
   ```bash
   echo $OTEL_EXPORTER_OTLP_ENDPOINT
   ```

2. Verify endpoint is reachable:
   ```bash
   curl http://localhost:4317
   ```

3. Check logs for export errors:
   ```go
   shutdown, err := instrumentation.SetupOTelSDKWithOptions(ctx, "service", "v1", logger)
   // Look for "failed to create OTLP trace exporter" in logs
   ```

4. Ensure shutdown is called:
   ```go
   defer shutdown(ctx)  // Flushes pending traces
   ```

### Environment Variable Not Working

**Problem:** `OTEL_EXPORTER_OTLP_ENDPOINT` is ignored.

**Solution:** Ensure you're using the new API:

```go
// ❌ Wrong - deprecated API, different behavior
instrumentation.SetupOTelSDK(...)

// ✅ Correct - new API with env override
instrumentation.SetupOTelSDKWithOptions(...)
```

### Missing Trace Context in Logs

**Problem:** Logs don't show `trace_id` and `span_id`.

**Solution:** Use `GetLogSpan` instead of separate logger:

```go
// ❌ Wrong
logger := logger.MustLogrFromContext(ctx)
logger.Info("message")

// ✅ Correct
ctx, spanLogger, end := instrumentation.GetLogSpan(ctx, "operation")
defer end()
spanLogger.Info("message")  // Includes trace_id and span_id
```

---

## Advanced Topics

### Custom Resource Attributes

```go
shutdown, err := instrumentation.SetupOTelSDKWithOptions(
    ctx, "service", "v1", logger,
    instrumentation.WithResourceAttributes(
        "service.namespace", "production",
        "service.instance.id", hostname,
        "deployment.environment", "us-west-2",
        "k8s.pod.name", podName,
        "k8s.namespace.name", namespace,
    ),
)
```

### Multiple Services in Same Process

```go
// Service 1
shutdown1, _ := instrumentation.SetupOTelSDKWithOptions(
    ctx, "api-gateway", "v1", logger,
    instrumentation.WithDefaultOTLPEndpoint("http://collector:4317"),
)
defer shutdown1(ctx)

// Service 2 uses global tracer set by first setup
// Just use GetLogSpan normally
```

### Sampling Configuration

OpenTelemetry SDK uses environment variables for sampling:

```bash
# Always sample
export OTEL_TRACES_SAMPLER=always_on

# Never sample
export OTEL_TRACES_SAMPLER=always_off

# Probabilistic sampling (10%)
export OTEL_TRACES_SAMPLER=traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1
```

---

## See Also

- [Logger Configuration API](./logger-configuration-api.md) - Logging configuration
- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
