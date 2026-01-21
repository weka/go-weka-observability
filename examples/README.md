# Examples

This directory contains example programs demonstrating various features of the go-weka-observability library. Each example is a standalone Go program that can be run directly.

## SpanLogger API Overview

The go-weka-observability library provides three API functions for working with spans and logging:

### 1. `CreateLogSpan(ctx, name, ...keysAndValues)` - Creating Owned Spans
**When to use:** You're creating a new operation that should be tracked as a child span.

```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation_name", "user_id", 123)
defer logger.End() // Required!

logger.Info("Operation started")
```

**Key points:**
- Returns `*SpanLogger` that you **own**
- You **MUST** call `defer logger.End()`
- New span becomes child of current span in context
- All logs enriched with trace/span IDs

### 2. `CurrentSpanLogger(ctx)` - Borrowing Current Span
**When to use:** Helper functions need to log under the current span without creating a new one.

```go
view := instrumentation.CurrentSpanLogger(ctx)
view.Info("Helper function working")
// NO End() call - you don't own this span!
```

**Key points:**
- Returns `*SpanLoggerView` (borrowed reference)
- **CANNOT** call `End()` - compile-time safety!
- No new span created - logs go to current span
- Perfect for utility functions and helpers

### 3. `CreateRootLogSpan(ctx, name, ...keysAndValues)` - Breaking Parent Chain
**When to use:** Starting a new independent trace (background jobs, async operations).

```go
ctx, logger := instrumentation.CreateRootLogSpan(ctx, "background_job", "job_id", "abc")
defer logger.End() // Required!

logger.Info("Independent trace with new trace ID")
```

**Key points:**
- Returns `*SpanLogger` that you **own**
- Creates **NEW trace ID** (breaks parent chain)
- Independent from any existing span in context
- Still must call `defer logger.End()`

## Error Handling Patterns

### `Error(err, msg, ...)` - Recoverable Errors
Use when the error is recoverable and the operation can continue:
```go
err := checkCache(ctx, "user-123")
if err != nil {
    logger.Error(err, "Cache miss - will fetch from database")
    // Operation continues, span status remains OK
}
```

### `SetError(err, msg, ...)` - Critical Errors
Use when the error represents operation failure:
```go
err := validateUserInput(ctx, input)
if err != nil {
    logger.SetError(err, "Validation failed")
    return // Operation cannot continue, span marked as ERROR
}
```

## Available Examples

### 1. Basic Example (`basic/`)
**Purpose:** Simple introduction to basic logging and tracing with nested function calls.

**Run with:**
```bash
go run ./examples/basic
```

**Demonstrates:**
- Basic `CreateLogSpan()` usage
- Nested span creation
- Context propagation between functions
- OpenTelemetry SDK setup

### 2. HTTP Tracing Example (`http_tracing/`)
**Purpose:** Comprehensive HTTP client/server distributed tracing demonstration.

**Run with:**
```bash
go run ./examples/http_tracing
```

**Demonstrates:**
- All three API functions (`CreateLogSpan`, `CurrentSpanLogger`, `CreateRootLogSpan`)
- HTTP server with tracing middleware
- HTTP client with automatic trace propagation
- Distributed tracing across service boundaries
- Trace context extraction and injection
- Multiple HTTP endpoints with different patterns

### 3. Span Lifecycle Example (`span_lifecycle/`)
**Purpose:** Educational example showing all three API functions side-by-side.

**Run with:**
```bash
go run ./examples/span_lifecycle
```

**Demonstrates:**
- **Section 1:** `CreateLogSpan()` for owned spans with parent-child relationships
- **Section 2:** `CurrentSpanLogger()` for borrowed spans in helper functions
- **Section 3:** `CreateRootLogSpan()` for independent traces in background jobs

**Best for:** Learning the differences between the three API functions.

### 4. Error Patterns Example (`error_patterns/`)
**Purpose:** Demonstrates proper error handling with `Error()` vs `SetError()`.

**Run with:**
```bash
go run ./examples/error_patterns
```

**Demonstrates:**
- **Pattern 1:** `Error()` for recoverable errors (cache miss, retries, optional features)
- **Pattern 2:** `SetError()` for critical errors (validation failure, database errors)
- **Pattern 3:** Complex workflows with mixed error severities
- When span status should be OK vs ERROR in tracing UIs

**Best for:** Understanding when to use each error logging method.

## Migration Guide

### Migrating from `GetLogSpan()` (Deprecated)

The old `GetLogSpan()` function has been replaced with three distinct functions for clarity and type safety.

**Old API (Deprecated):**
```go
ctx, logger, end := instrumentation.GetLogSpan(ctx, "operation")
defer end()
```

**Choose the right replacement:**

#### 1. For Creating New Spans (Most Common)
**Before:**
```go
ctx, logger, end := instrumentation.GetLogSpan(ctx, "operation")
defer end()
```

**After:**
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
defer logger.End()
```

#### 2. For Logging Under Current Span (Helper Functions)
**Before:**
```go
// If you only needed to log, not create a new span
ctx, logger, end := instrumentation.GetLogSpan(ctx, "")
defer end()
logger.Info("Helper function")
```

**After:**
```go
// No new span created - just borrows the current one
view := instrumentation.CurrentSpanLogger(ctx)
view.Info("Helper function")
// No End() call needed!
```

#### 3. For Independent Traces (Background Jobs)
**Before:**
```go
// Had to manually break parent chain
ctx = context.Background()
ctx, logger, end := instrumentation.GetLogSpan(ctx, "background_job")
defer end()
```

**After:**
```go
// Automatically creates new trace ID
ctx, logger := instrumentation.CreateRootLogSpan(ctx, "background_job")
defer logger.End()
```

### Why Three Functions?

**The Problem:** `GetLogSpan()` was doing too many things:
- Creating spans vs borrowing existing spans (unclear)
- Hard to tell if you need to call `end()` or not
- No compile-time safety for span ownership

**The Solution:** Three functions with clear semantics:
1. **`CreateLogSpan`** - You own it, you end it (enforced by type system)
2. **`CurrentSpanLogger`** - You borrow it, can't end it (compile-time error if you try)
3. **`CreateRootLogSpan`** - You own it, new trace (explicit about breaking parent chain)

## Best Practices

### 1. Span Ownership and Lifecycle
✅ **DO:** Always use `defer logger.End()` immediately after creating a span:
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
defer logger.End() // Ensures span closes even on early returns/panics
```

❌ **DON'T:** Forget to call `End()` or call it manually without defer:
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
// ... code ...
logger.End() // BAD: Won't execute if panic or early return happens
```

### 2. Choosing the Right API Function
✅ **DO:** Use `CurrentSpanLogger()` in helper functions that don't need a new span:
```go
func validateInput(ctx context.Context, input string) error {
    view := instrumentation.CurrentSpanLogger(ctx)
    view.Info("Validating input")
    // Just logging - no new span needed
}
```

❌ **DON'T:** Create unnecessary spans in every helper function:
```go
func validateInput(ctx context.Context, input string) error {
    ctx, logger := instrumentation.CreateLogSpan(ctx, "validate_input")
    defer logger.End()
    // Overkill if just logging a few lines
}
```

### 3. Parent-Child Span Relationships
✅ **DO:** Pass the updated context to maintain parent-child relationships:
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "parent")
defer logger.End()

childFunction(ctx) // Pass updated context
```

❌ **DON'T:** Pass the original context, breaking the trace chain:
```go
originalCtx := ctx
ctx, logger := instrumentation.CreateLogSpan(ctx, "parent")
defer logger.End()

childFunction(originalCtx) // BAD: Child won't be linked to parent
```

### 4. Error Handling
✅ **DO:** Use `Error()` for recoverable errors, `SetError()` for critical failures:
```go
// Recoverable - cache miss is expected
if err := checkCache(ctx); err != nil {
    logger.Error(err, "Cache miss, using fallback")
}

// Critical - validation must succeed
if err := validate(ctx); err != nil {
    logger.SetError(err, "Validation failed")
    return err
}
```

❌ **DON'T:** Use `SetError()` for everything - it marks spans as failed:
```go
if err := checkCache(ctx); err != nil {
    logger.SetError(err, "Cache miss") // BAD: Span shows as failed for normal operation
}
```

### 5. Root Spans for Background Jobs
✅ **DO:** Use `CreateRootLogSpan()` for independent operations:
```go
go func(parentCtx context.Context) {
    ctx, logger := instrumentation.CreateRootLogSpan(parentCtx, "background_job")
    defer logger.End()
    // This trace is independent from the parent request
}(ctx)
```

❌ **DON'T:** Use `CreateLogSpan()` if you want independent traces:
```go
go func(parentCtx context.Context) {
    ctx, logger := instrumentation.CreateLogSpan(parentCtx, "background_job")
    defer logger.End()
    // BAD: This is still part of the parent trace
}(ctx)
```

## Environment Variables

All examples support the following environment variables for configuration:

- `LOG_LEVEL`: Set to "0" for trace level, "1" for debug, "2" for info (default varies by example)
- `LOG_FORMAT`: Set to "json", "plain", or "raw" (default varies by example)
- `LOG_CALLER_DIR_LVL`: Number of directory levels to show in caller info (default: "1")
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint (optional)

## Running with OpenTelemetry Collector

To export traces to an OpenTelemetry collector, set the endpoint environment variable:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
go run ./examples/basic
```

## Example Structure

Each example is organized as a standalone Go main package in its own directory:

```
examples/
├── README.md              # This file
├── basic/                 # Basic span creation and context propagation
│   └── main.go
├── http_tracing/          # HTTP distributed tracing with all three API functions
│   └── main.go
├── span_lifecycle/        # Educational: all three API functions side-by-side
│   └── main.go
└── error_patterns/        # Error() vs SetError() demonstration
    └── main.go
```

This structure allows you to:
- Run examples directly with `go run ./examples/[example_name]`
- Copy example directories as starting points for your own projects
- Understand different usage patterns of the observability library
- Learn the differences between the three span API functions
