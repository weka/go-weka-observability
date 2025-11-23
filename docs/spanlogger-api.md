# SpanLogger API: Type-Safe Span Ownership

## Problem & Solution

**Problem**: The previous `GetLogSpan()` API had confusing overloaded behavior:
- Empty string parameter meant "use current span" (don't create new one)
- Non-empty string parameter meant "create new span"
- Return values were inconsistent (sometimes end function was no-op)
- No compile-time safety to prevent calling `End()` on borrowed spans
- Unclear ownership semantics led to potential resource leaks

**Solution**: Three distinct types with clear ownership semantics:
- `SpanLogger` - Owned span that MUST be ended
- `SpanLoggerView` - Borrowed span that CANNOT be ended (compile-time safety)
- Three explicit API functions replacing the ambiguous single function

## Architecture

### Core Types

#### `spanLoggerBase` (internal)
**Purpose**: Shared functionality for both owned and borrowed spans
- **Why it exists**: Eliminates code duplication between `SpanLogger` and `SpanLoggerView`
- **Key responsibility**: Provides all logging methods (`Info`, `Debug`, `Warn`, `Error`, etc.)
- **Design note**: Not exported - implementation detail

**Fields:**
```go
ctx         context.Context  // Enriched context with logger and span
logr.Logger                  // Embedded public logging interface
trace.Span                   // Embedded public span interface
```

#### `SpanLogger` (owned span)
**Purpose**: Represents a span you created and own
- **Why it exists**: Type-safe span ownership - compiler enforces calling `End()`
- **Key responsibility**: Must be ended to prevent resource leaks
- **Validation**: Created only by `CreateSpan()` or `CreateRootSpan()`

**Fields:**
```go
*spanLoggerBase
shutdown func()  // Cleanup function (never nil)
```

**Methods:**
- `End()` - MUST be called (typically via `defer logger.End()`)
- `WithValues(keysAndValues ...any)` - Returns new `*SpanLogger` with enriched context
- All logging methods inherited from `spanLoggerBase`

#### `SpanLoggerView` (borrowed span)
**Purpose**: Represents a borrowed span from context
- **Why it exists**: Compile-time safety - cannot accidentally call `End()` on borrowed span
- **Key responsibility**: Log under current span without ownership concerns
- **Validation**: Cannot end the span (NO `End()` method)

**Fields:**
```go
*spanLoggerBase  // Only contains shared base
```

**Methods:**
- `WithValues(keysAndValues ...any)` - Returns new `*SpanLoggerView` with enriched context
- All logging methods inherited from `spanLoggerBase`
- **Notably MISSING**: `End()` method (compile-time safety)

### Design Decisions

#### Why Custom Types Instead of Interfaces?
**Decision**: Use concrete types (`SpanLogger`, `SpanLoggerView`) instead of interfaces
**Rationale**:
- Compile-time enforcement: `SpanLoggerView` literally cannot have `End()` method
- Interface-based approach would require runtime checks or panic
- Follows primitive obsession avoidance principle - domain types with self-validating behavior
- Clear ownership semantics visible in function signatures

#### Why Embedded Types?
**Decision**: Embed `logr.Logger` and `trace.Span` in `spanLoggerBase`
**Rationale**:
- Provides direct access to full Logger and Span interfaces
- No need to wrap every method
- Allows adding/overriding specific methods (Info, Debug, Warn) for span event integration
- Cohesion: Logger and Span are tightly coupled in observability

#### Why Separate API Functions?
**Decision**: Three functions (`CreateSpan`, `CreateRootSpan`, `CurrentSpanLogger`) instead of one
**Rationale**:
- Explicit intent: Function name reveals what you're doing
- Type safety: Return types enforce correct usage patterns
- No magic behavior: No empty string special cases
- Self-documenting code: `CurrentSpanLogger(ctx)` is clearer than `GetLogSpan(ctx, "")`

### Data Flow

```
User Code:
    CreateSpan(ctx, "operation", "key", "value")
            ↓
    Validation (even number of key-value pairs)
            ↓
    Logger Enrichment (add operation name + key-values)
            ↓
    Span Creation (via OpenTelemetry)
            ↓
    Logger Enhancement (add trace_id, span_id)
            ↓
    Return: (enriched context, *SpanLogger)
            ↓
    User: defer logger.End()
            ↓
    Logging: logger.Info("msg")
            ├─> Logger outputs to configured sink
            └─> Span.AddEvent(msg) records in trace
            ↓
    End: logger.End()
            ├─> Logs operation completion
            └─> Closes span (exports to OTel backend)
```

### Integration Points

**Consumed by:**
- Application code that needs structured logging + tracing
- HTTP handlers, background jobs, service methods
- Any code that wants unified observability (logs + traces)

**Depends on:**
- `github.com/go-logr/logr` - Structured logging interface
- `go.opentelemetry.io/otel/trace` - OpenTelemetry tracing
- `github.com/weka/go-weka-observability/logger` - Logger initialization

**Events/Hooks:**
- Logging methods automatically create span events
- Errors recorded as span exceptions with `RecordError()`
- `SetError()` additionally sets span status to Error

## Usage

### Basic Usage - Creating Owned Spans

```go
package main

import (
    "context"
    "github.com/weka/go-weka-observability/instrumentation"
)

func processRequest(ctx context.Context, userID int) error {
    // Create span - returns owned *SpanLogger
    ctx, logger := instrumentation.CreateSpan(ctx, "process_request",
        "user_id", userID)
    defer logger.End()  // REQUIRED - compiler helps enforce this

    logger.Info("Starting request processing")

    // Span is automatically parent of any child spans
    if err := validateUser(ctx, userID); err != nil {
        logger.SetError(err, "User validation failed")
        return err
    }

    logger.Info("Request processed successfully")
    return nil
}
```

### Using Borrowed Spans (Views)

```go
func helperFunction(ctx context.Context) {
    // Get view of current span - returns *SpanLoggerView
    view := instrumentation.CurrentSpanLogger(ctx)

    view.Info("Helper function called")
    view.Debug("Processing data", "count", 42)

    // view.End()  // COMPILE ERROR - no End() method!
    // This prevents accidental resource leaks
}
```

### Creating Root Spans (Breaking Parent Chain)

```go
func backgroundJob(ctx context.Context, jobID string) {
    // Create root span - starts new trace
    ctx, logger := instrumentation.CreateRootSpan(ctx, "background_job",
        "job_id", jobID)
    defer logger.End()

    logger.Info("Background job started")
    // This span has its own trace ID, independent of caller

    processJob(ctx, jobID)
}
```

### Advanced Scenarios

#### Enriching Context with WithValues

```go
func processUser(ctx context.Context, userID int, tenant string) error {
    ctx, logger := instrumentation.CreateSpan(ctx, "process_user")
    defer logger.End()

    // Enrich logger and context with additional values
    ctx, enrichedLogger := logger.WithValues("user_id", userID, "tenant", tenant)

    // All subsequent logs from enrichedLogger include user_id and tenant
    enrichedLogger.Info("Processing user")

    // Pass enriched context to children
    return processUserData(ctx, userID)
}
```

#### Error Handling Patterns

```go
func operationWithErrors(ctx context.Context) error {
    ctx, logger := instrumentation.CreateSpan(ctx, "operation")
    defer logger.End()

    // Log error without marking span as failed
    if err := tryRecoverable(); err != nil {
        logger.Error(err, "Recoverable error occurred")
        // Span status remains OK, but error is recorded in span events
    }

    // Log error AND mark span as failed
    if err := tryMustSucceed(); err != nil {
        logger.SetError(err, "Critical error occurred")
        // Span status = Error, visible in tracing UI
        return err
    }

    return nil
}
```

#### Nested Spans with Parent-Child Relationships

```go
func outerOperation(ctx context.Context) error {
    ctx, outerLogger := instrumentation.CreateSpan(ctx, "outer_operation")
    defer outerLogger.End()

    outerLogger.Info("Outer operation started")

    // Child span automatically links to parent
    ctx, innerLogger := instrumentation.CreateSpan(ctx, "inner_operation")
    innerLogger.Info("Inner operation started")
    innerLogger.End()

    outerLogger.Info("Outer operation completed")
    return nil
}
```

## Migration from GetLogSpan

### Case 1: Creating New Span (Most Common)

**Before:**
```go
ctx, logger, end := instrumentation.GetLogSpan(ctx, "operation", "key", "value")
defer end()

logger.Info("Processing")
```

**After:**
```go
ctx, logger := instrumentation.CreateSpan(ctx, "operation", "key", "value")
defer logger.End()

logger.Info("Processing")
```

### Case 2: Using Current Span (Empty String)

**Before:**
```go
_, logger, _ := instrumentation.GetLogSpan(ctx, "")  // Empty string = magic behavior
logger.Info("Logging under current span")
```

**After:**
```go
view := instrumentation.CurrentSpanLogger(ctx)
view.Info("Logging under current span")
```

### Case 3: Creating Root Span

**Before:**
```go
// Custom implementation with trace.WithNewRoot()
spanCtx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(ctx))
ctx, logger, end := customRootSpanCreation(spanCtx, "root_operation")
defer end()
```

**After:**
```go
ctx, logger := instrumentation.CreateRootSpan(ctx, "root_operation")
defer logger.End()
```

## Testing Strategy

### Unit Tests
**Covered**: All three API functions and both span types
- `TestCreateSpan_CreatesOwnedSpan` - Verifies span ownership and End() requirement
- `TestCreateSpan_CreatesChildSpan` - Validates parent-child relationships
- `TestCreateRootSpan_BreaksParentChain` - Confirms independent trace IDs
- `TestCurrentSpanLogger_ReturnsBorrowedSpan` - Validates view behavior
- `TestSpanLoggerLoggingMethods` - Verifies all logging methods work and create span events
- `TestSpanLoggerViewLoggingMethods` - Ensures view can log but not end
- `TestSpanLoggerErrorMethods` - Distinguishes Error() vs SetError() behavior
- `TestSpanLoggerSetAttributes` - Validates attribute setting
- `TestSpanLoggerWithValues` - Tests context enrichment

**Approach**:
- Using `testify/suite` for organized test structure
- `tracetest.SpanRecorder` for span verification (official OTel testing utility)
- `funcr.NewJSON` with capture buffer for logger call verification
- Dual verification: Tests confirm BOTH span events AND logger output

### Integration Tests
**Covered**: Logger and span integration
- Tests verify span events match logged messages
- Tests confirm logger enrichment (trace_id, span_id) appears in output
- Tests validate attribute propagation from logs to spans

### Coverage
**Percentage**: 48.9% (instrumentation package)
**Rationale**:
- Core API functions: 100% covered
- All public methods on SpanLogger/SpanLoggerView: 100% covered
- Helper functions (internal): Partially covered (tested indirectly through API)
- Focus on public API contract, not implementation details

## Future Considerations

### Known Limitations
- No streaming/continuous logging support (each call is discrete)
- No automatic sampling configuration (relies on OTel SDK setup)
- Panic in span creation if Tracer not initialized (could return error instead)

### Potential Extensions
- Add `SpanLoggerWithSampling` for fine-grained sampling control per span
- Support for span links (connecting spans across trace boundaries)
- Automatic metric generation from span durations
- Integration with `slog` (Go 1.21+ structured logging)

### Related Features
- Metrics collection using same context propagation pattern
- HTTP middleware for automatic span creation per request
- gRPC interceptors for automatic span creation per RPC
- Database query instrumentation with automatic spans

## References

### Related Packages
- `github.com/weka/go-weka-observability/logger` - Logger initialization and configuration
- `go.opentelemetry.io/otel/trace` - OpenTelemetry tracing primitives
- `github.com/go-logr/logr` - Structured logging interface

### External Documentation
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [go-logr Documentation](https://github.com/go-logr/logr)
- [Semantic Conventions for Spans](https://opentelemetry.io/docs/specs/semconv/general/trace/)

### Design Patterns Used
- **Vertical Slice Architecture**: All span logger logic grouped together
- **Self-Validating Types**: `SpanLogger` and `SpanLoggerView` enforce ownership at compile-time
- **Primitive Obsession Avoidance**: Custom types instead of string-based flags
- **Embedded Composition**: Reuses Logger and Span interfaces through embedding
