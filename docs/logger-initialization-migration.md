# Logger Initialization Migration Guide

This document demonstrates how to migrate from the deprecated `GetLoggerForContext` API to the new, cleaner logger initialization pattern.

## Overview

The old API had confusing triple behavior based on nil pointer checks and unclear naming. The new API provides explicit, single-purpose functions that are easier to understand and use correctly.

## Quick Migration Reference

Use this table to quickly find the new equivalent for deprecated functions:

| Old API (Deprecated) | New API (Use Instead) | Notes |
|---------------------|----------------------|-------|
| `GetLoggerForContext(ctx, nil, name)` | `LogrFromContextOrDefault(ctx).WithName(name)` | Direct equivalent - retrieves or creates default |
| `GetLoggerForContext(ctx, nil, name)` (for app startup) | `CreateLogger()` + `ContextWithLogr()` | Explicit control - recommended for startup |
| `GetLoggerForContext(ctx, &baseLogger, name)` | `baseLogger.WithName(name)` + `ContextWithLogr()` | When you have existing logger |
| `NewZeroLogger()` | `CreateLogger()` | Returns `logr.Logger` instead of `*zerolog.Logger` |
| `NewZeroLoggerWithConfig(config)` | `CreateLoggerFrom(config)` | Config structure has changed |
| `zerologr.New(&zlog)` | `CreateLogger()` | No manual wrapping needed |
| `SetupOTelSDK(ctx, name, ver, logger)` | `SetupOTelSDKWithOptions(ctx, name, ver, logger)` | Use functional options for endpoint config |
| `ContextWithLogger(ctx, logger)` | `ContextWithLogr(ctx, logger)` | Standard Go naming |
| `GetLoggerFromContext(ctx)` | `MustLogrFromContext(ctx)` or `LogrFromContext(ctx)` | Choose based on error handling needs |
| `GetStderrWriter()` | `GetStderrWriterFromFormat(formatConfig)` | Takes explicit config |
| `GetMultiLevelWriter()` | `GetMultiLevelWriterWithConfig(config)` | Takes explicit config |

**Key principle**: The new API uses **functional options** (`With*` functions) for configuration instead of config structs for simple cases.

## Old Way (Deprecated)

```go
import (
    "github.com/go-logr/zerologr"
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

// Confusing: passing nil pointer, double logger creation
logr := zerologr.New(logger.NewZeroLogger())
ctx, ctxLogger := instrumentation.GetLoggerForContext(ctx, &logr, tracerName)

shutdownFn, err := instrumentation.SetupOTelSDK(ctx, tracerName, version, ctxLogger)
if err != nil {
    return nil, err
}
```

**Problems with the old approach:**
- Requires creating logger then immediately wrapping in pointer
- Function name doesn't describe what it actually does
- Triple behavior based on nil checks is confusing
- Mixes logger creation with context storage

## New Way (Recommended)

```go
import (
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

// Create logger from config
logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())

// Store in context
ctx = logger.ContextWithLogr(ctx, logr)

// Use logger directly (no retrieval needed!)
shutdownFn, err := instrumentation.SetupOTelSDK(ctx, tracerName, version, logr)
if err != nil {
    return nil, err
}
```

**Benefits of the new approach:**
- Proper separation of concerns (logger package owns logger lifecycle)
- No redundant context retrieval
- Clear package boundaries
- Explicit configuration control
- No pointer anti-patterns
- Idiomatic Go naming
- Environment-aware by default

## Understanding the Old API's Confusing Behavior

The old `GetLoggerForContext` function had two different behaviors based on the `baseLogger` parameter. Understanding this will help you migrate correctly.

### Behavior 1: baseLogger=nil → Retrieve or Create

**When:** `baseLogger=nil` (most common usage)

The function would:
1. Try to retrieve logger from context
2. If not found, create a default logger
3. Return the logger

**Old pattern:**
```go
// Either reuses existing logger OR creates new one (hidden behavior!)
ctx, logger := instrumentation.GetLoggerForContext(ctx, nil, "name")
```

**New pattern - Direct equivalent:**
```go
// Does the SAME thing - retrieves from context or creates default
logger := logger.LogrFromContextOrDefault(ctx).WithName("name")
```

**New pattern - Explicit control (recommended for application startup):**
```go
// Explicitly create fresh logger and store it
logr := logger.CreateLogger()
ctx = logger.ContextWithLogr(ctx, logr)
logger := logr.WithName("name")
```

**Key difference:** `LogrFromContextOrDefault` is the direct replacement for the old behavior. For application startup where you want explicit control, use `CreateLogger()`.

---

### Behavior 2: baseLogger provided → Use Provided Logger

**When:** `baseLogger` is NOT nil (less common, pointer anti-pattern)

**Old pattern:**
```go
// Explicitly pass logger to use (pointer anti-pattern)
existingLogger := zerologr.New(logger.NewZeroLogger())
ctx, logger := instrumentation.GetLoggerForContext(ctx, &existingLogger, "name")
```

**New pattern:**
```go
// Create, store, and use logger (no pointers needed)
logr := logger.CreateLogger()
ctx = logger.ContextWithLogr(ctx, logr)
logger := logr.WithName("name")
```

**Key difference:** No more pointer anti-pattern. All loggers are stored in context the same way.

---

**Why the old API was confusing:**
- Same function call (`GetLoggerForContext(ctx, nil, "name")`) could either **create** or **reuse** a logger depending on hidden context state
- Pointer vs nil distinction (`&logger` vs `nil`) changed behavior drastically
- Impossible to tell from code whether logger is new or reused without tracing execution

**Why the new API is better:**
- **Direct equivalent**: `LogrFromContextOrDefault(ctx)` does exactly what `GetLoggerForContext(ctx, nil, ...)` did
- **Explicit control**: `CreateLogger()` + `ContextWithLogr()` when you want explicit logger creation
- **Clear naming**: Function names describe exactly what they do
- **No pointers**: No confusing pointer/nil patterns

## Understanding the API Flow

When you see this migration pattern, it might look confusing:

```go
logr := logger.CreateLogger(...)
ctx = logger.ContextWithLogr(ctx, logr)              // Why is this needed?
shutdownFn, err := SetupOTelSDKWithOptions(ctx, name, version, logr)
```

**Question**: "Why do I need to call `ContextWithLogr` if I'm already passing `logr` to `SetupOTelSDKWithOptions`? Isn't that redundant?"

**Answer**: No! Here's what each step does:

### Step-by-Step Breakdown

```go
// STEP 1: Create logger instance
logr := logger.CreateLogger(
    logger.WithInfoLevel(),
    logger.WithConsoleSink(),
)

// STEP 2: Store logger in context
ctx = logger.ContextWithLogr(ctx, logr)
// → This is REQUIRED so GetLogSpan can retrieve it later
// → NOT required by SetupOTelSDKWithOptions itself

// STEP 3: Setup OpenTelemetry
shutdownFn, err := SetupOTelSDKWithOptions(ctx, name, version, logr)
// → Uses the logr PARAMETER for logging during SDK initialization
// → Does NOT retrieve logger from context
// → Does NOT store logger in context

// STEP 4: Later in your application code
ctx, spanLogger, end := instrumentation.GetLogSpan(ctx, "operation")
defer end()
// → Retrieves logger FROM CONTEXT (stored in Step 2)
// → NOT from what was passed to SetupOTelSDKWithOptions
```

### Order Flexibility: Steps 2 and 3 Can Be Swapped

**IMPORTANT**: The order between `ContextWithLogr()` (Step 2) and `SetupOTelSDKWithOptions()` (Step 3) does NOT matter!

You can call them in either order:

```go
// Option A: Recommended (SetupOTelSDK first)
logr := logger.CreateLogger()
shutdownFn, err := SetupOTelSDKWithOptions(ctx, name, version, logr)
ctx = logger.ContextWithLogr(ctx, logr)  // Can be done after SetupOTelSDK

// Option B: Alternative (ContextWithLogr first)
logr := logger.CreateLogger()
ctx = logger.ContextWithLogr(ctx, logr)  // Can be done before SetupOTelSDK
shutdownFn, err := SetupOTelSDKWithOptions(ctx, name, version, logr)
```

**Why?** Because `SetupOTelSDKWithOptions` uses the `logr` **parameter** only - it never retrieves the logger from context. The only requirement is that `ContextWithLogr()` must be called **before** `GetLogSpan()` is used anywhere in your application.

### Why This Design?

**`SetupOTelSDKWithOptions` uses the parameter:**
- You call it once during application startup
- It needs a logger to log SDK initialization messages
- Taking it as a parameter makes this explicit

**`GetLogSpan` uses context:**
- You call it throughout your application (handlers, business logic, etc.)
- You don't want to pass logger as a parameter to every function
- Context storage makes it available everywhere without parameter passing

### The Confusion Explained

It **looks** like `ContextWithLogr` is needed for `SetupOTelSDKWithOptions` because you often see them called one after the other, but that's just convention, not a dependency. You can call them in any order.

**Timeline:**
1. **Startup**: Create logger → (Setup OTel + Store in context in any order)
2. **Runtime**: Call `GetLogSpan` (retrieves logger from context)

The `ContextWithLogr` call happens at startup, but it's **preparing for runtime** when `GetLogSpan` is called. It has nothing to do with `SetupOTelSDKWithOptions`.

### Complete Flow Example

```go
func main() {
    ctx := context.Background()

    // Startup: Initialize observability stack
    logr := logger.CreateLogger(logger.WithInfoLevel())

    // Recommended pattern: SetupOTelSDK first, then ContextWithLogr
    shutdown, err := instrumentation.SetupOTelSDKWithOptions(
        ctx, "my-service", "v1.0.0", logr,   // Uses logr for setup logging
    )
    if err != nil {
        panic(err)
    }
    defer shutdown(ctx)

    ctx = logger.ContextWithLogr(ctx, logr)  // For GetLogSpan later ↓
    // Note: Could also be called before SetupOTelSDK - order doesn't matter

    // Runtime: Handle requests
    processRequest(ctx)  // Logger is in context, no need to pass it
}

func processRequest(ctx context.Context) {
    // GetLogSpan retrieves logger from context (stored during startup)
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "process_request")
    defer end()

    logger.Info("Processing request", "user_id", 123)

    // Nested operations also use logger from context
    queryDatabase(ctx)
}

func queryDatabase(ctx context.Context) {
    // Also retrieves logger from context - no parameter passing needed!
    ctx, logger, end := instrumentation.GetLogSpan(ctx, "query_database")
    defer end()

    logger.Info("Querying database")
}
```

**Key takeaway**: `ContextWithLogr` stores the logger for your **entire application** to use via `GetLogSpan`, not just for the next function call.

## Key Feature: LogrFromContextOrDefault

One of the most useful functions in the new API is `LogrFromContextOrDefault(ctx)` - it retrieves the logger from context, or creates a default logger if one doesn't exist.

**Why this matters:**
- Safe to call anywhere - never panics, never returns nil
- Perfect for middleware, helper functions, and library code
- Gracefully handles cases where logger might not be in context
- Creates default logger with environment configuration if needed

**Quick example:**
```go
// Works whether logger is in context or not
logger := logger.LogrFromContextOrDefault(ctx)
logger.Info("Processing request")  // Always works
```

**Use cases:**
- ✅ HTTP middleware that should work with or without logger setup
- ✅ Helper functions called from different contexts
- ✅ Library code that shouldn't require explicit logger initialization
- ✅ Test utilities that need flexible logger access

See **Scenario 5** below for complete examples and when to use each retrieval strategy.

## Common Migration Scenarios

This section covers real-world migration scenarios that users commonly encounter when upgrading.

### Scenario 1: GetLoggerForContext + SetupOTelSDK Pattern

**What you had:**
```go
ctx, ctxLogger := instrumentation.GetLoggerForContext(ctx, nil, name)
shutdownFn, err := instrumentation.SetupOTelSDK(ctx, name, version, ctxLogger)
```

**Understanding the old behavior:**
When you called `GetLoggerForContext(ctx, nil, name)`, it would:
1. Try to retrieve logger from context
2. If not found, create a default logger

This is **exactly** what `LogrFromContextOrDefault` does.

**Option 1: Direct equivalent (retrieve or create):**
```go
// Retrieves from context OR creates default (same as old behavior)
logr := logger.LogrFromContextOrDefault(ctx).WithName(name)
shutdownFn, err := instrumentation.SetupOTelSDKWithOptions(ctx, name, version, logr)
ctx = logger.ContextWithLogr(ctx, logr)  // Ensure stored for GetLogSpan
```

**Option 2: Explicit fresh logger (recommended for application startup):**
```go
// Create fresh logger with explicit configuration
logr := logger.CreateLogger(
    logger.WithConsoleSink(),
    logger.WithInfoLevel(),
).WithName(name)

shutdownFn, err := instrumentation.SetupOTelSDKWithOptions(ctx, name, version, logr)
ctx = logger.ContextWithLogr(ctx, logr)  // Store for GetLogSpan
```

**Why this works:**
- **Option 1**: `LogrFromContextOrDefault` is the direct replacement for `GetLoggerForContext(ctx, nil, ...)`
- **Option 2**: `CreateLogger()` gives explicit control over logger configuration
- `SetupOTelSDKWithOptions()` and `ContextWithLogr()` can be called in any order
- `ContextWithLogr()` stores logger for `GetLogSpan` to retrieve later
- Functional options pattern for any additional config (endpoints, attributes)

**Key differences:**
- No more `nil` pointer patterns
- Explicit context storage (more readable)
- Logger configuration via `With*` options (more flexible)
- Environment variables (`LOG_LEVEL`, `LOG_FORMAT`, etc.) still respected automatically

---

### Scenario 2: Custom Formatting (Raw, Plain, JSON)

**What you had:**
```go
zlog := zerologger.NewZeroLogger()
logr := zerologr.New(&zlog)
ctx, logger := instrumentation.GetLoggerForContext(ctx, &logr, name)
```

**What you need (if you want raw format):**
```go
// Create logger with explicit formatting options
logr := logger.CreateLogger(
    logger.WithConsoleSink(),
    logger.WithRawFormat(),  // or WithPlainFormat(), WithJSONFormat()
    logger.WithInfoLevel(),
)

// Store in context for GetLogSpan
ctx = logger.ContextWithLogr(ctx, logr)

// Retrieve and add name for immediate use
logger := logger.MustLogrFromContext(ctx).WithName(name)
```

**Available formatting options:**
- `WithJSONFormat()` - Structured JSON output (default)
- `WithRawFormat()` - Plain text, no timestamps
- `WithPlainFormat()` - Plain text with timestamps
- `WithTimeOnly()` - Use time-only format instead of full timestamp

**Why this works:**
- Functional options provide explicit control over formatting
- No need to manually create and wrap zerolog logger
- Environment variable `LOG_FORMAT=raw` still overrides if set

---

### Scenario 3: File Logging with Rotation

**What you had:**
```go
zlog := zerologger.NewZeroLoggerWithConfig(zerologger.Config{
    OutputMode:  zerologger.FileMode,
    LogDir:      "/var/log",
    LogFileName: "app.log",
})
logr := zerologr.New(&zlog)
ctx, logger := instrumentation.GetLoggerForContext(ctx, &logr, "")
```

**What you need:**
```go
// Create file logger with automatic rotation
logr := logger.CreateLogger(
    logger.WithFileSink("/var/log", "app.log"),
    logger.WithRotation(100, 5, 28),  // 100MB max size, 5 backups, 28 days retention
    logger.WithInfoLevel(),
)

// Store in context for GetLogSpan
ctx = logger.ContextWithLogr(ctx, logr)
```

**Why this works:**
- `WithFileSink()` replaces `OutputMode: FileMode` config
- `WithRotation()` sets up automatic log rotation (max size, max files, max age)
- Cleaner, more intuitive API
- Environment variables (`LOG_DIR`, `LOG_FILE_NAME`, `LOG_MAX_SIZE_MB`) still override if set

---

### Scenario 4: No Configuration (Just Defaults)

**What you had:**
```go
zlog := zerologger.NewZeroLogger()
logr := zerologr.New(&zlog)
ctx = instrumentation.ContextWithLogger(ctx, &logr)
```

**What you need:**
```go
// Create logger with sensible defaults (console, JSON, info level)
logr := logger.CreateLogger()

// Store in context for GetLogSpan
ctx = logger.ContextWithLogr(ctx, logr)
```

**Default behavior:**
- Console output (stderr)
- JSON format
- Info level
- Automatically respects `LOG_*` environment variables

**This is the simplest migration path** - just call `CreateLogger()` with no arguments.

---

### Scenario 5: Retrieve Logger or Create Default (Helper Functions, Middleware)

**Use case**: Code that might be called with or without logger in context (middleware, utilities, library code)

**What you had:**
```go
// Old API would create logger if missing (hidden behavior)
ctx, logger := instrumentation.GetLoggerForContext(ctx, nil, "")
logger.Info("Processing request")
```

**What you need:**
```go
// Retrieves from context, or creates default if missing (explicit)
logger := logger.LogrFromContextOrDefault(ctx)
logger.Info("Processing request")

// Or with a name:
logger := logger.LogrFromContextOrDefault(ctx).WithName("my-helper")
```

**Complete example - HTTP middleware:**
```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Safe: works whether logger is in context or not
        logger := logger.LogrFromContextOrDefault(r.Context())
        logger.Info("Request received",
            "method", r.Method,
            "path", r.URL.Path,
        )
        next.ServeHTTP(w, r)
    })
}
```

**Why this works:**
- Safe to call anywhere - never panics, never returns nil
- Creates default logger with environment config if not found
- Perfect for library code, utilities, middleware

**When to use:**
- ✅ Helper functions that may be called from different contexts
- ✅ Middleware that should work with or without logger setup
- ✅ Library code that shouldn't require explicit logger initialization
- ❌ Application startup code (use explicit `CreateLogger` + `ContextWithLogr`)
- ❌ Code that REQUIRES a logger (use `MustLogrFromContext` to fail fast)

## Migration Patterns

### Pattern 1: Default Logger Initialization

**Old:**
```go
import (
    "github.com/go-logr/zerologr"
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

logr := zerologr.New(logger.NewZeroLogger())
ctx, logger := instrumentation.GetLoggerForContext(ctx, &logr, "my-service")
```

**New:**
```go
import "github.com/weka/go-weka-observability/logger"

logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())
ctx = logger.ContextWithLogr(ctx, logr)
logger := logr.WithName("my-service")
```

### Pattern 2: File Logger Initialization

**Old:**
```go
import (
    "github.com/go-logr/zerologr"
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

zlog := logger.NewZeroLoggerWithConfig(logger.Config{
    OutputMode:  logger.FileMode,
    LogDir:      "/var/log",
    LogFileName: "app.log",
})
logr := zerologr.New(zlog)
ctx, logger := instrumentation.GetLoggerForContext(ctx, &logr, "my-service")
```

**New:**
```go
import "github.com/weka/go-weka-observability/logger"

logr := logger.CreateLoggerFrom(logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/var/log",
        FileName:   "app.log",
        MaxSizeMB:  100,
        MaxFiles:   5,
        MaxAgeDays: 28,
    },
})
ctx = logger.ContextWithLogr(ctx, logr)
logger := logr.WithName("my-service")
```

### Pattern 3: Existing Logger Storage

**Old:**
```go
import (
    "github.com/go-logr/zerologr"
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

existingLogger := zerologr.New(logger.NewZeroLogger())
ctx, _ := instrumentation.GetLoggerForContext(ctx, &existingLogger, "")
```

**New:**
```go
import "github.com/weka/go-weka-observability/logger"

logr := logger.CreateLoggerFrom(logger.DefaultConfig()).WithName("my-service")
ctx = logger.ContextWithLogr(ctx, logr)
```

### Pattern 4: Graceful Logger Retrieval

**Old:**
```go
// No graceful error handling pattern existed
```

**New:**
```go
import "github.com/weka/go-weka-observability/logger"

log, err := logger.LogrFromContext(ctx)
if err != nil {
    // Handle missing logger - create default or return error
    log = logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())
    ctx = logger.ContextWithLogr(ctx, log)
}
```

## New API Reference

All logger operations are now in the `logger` package:

```go
import "github.com/weka/go-weka-observability/logger"
```

### Creation Functions

- **`logger.CreateLoggerFrom(config)`** - Creates new logr.Logger with config

### Context Functions

- **`logger.ContextWithLogr(ctx, logr)`** - Stores logger in context

### Retrieval Functions

- **`logger.LogrFromContext(ctx)`** - Returns logger and error (for graceful handling)
- **`logger.MustLogrFromContext(ctx)`** - Returns logger or panics (when logger is required)
- **`logger.LogrFromContextOrDefault(ctx)`** - Returns logger or creates default (never fails)

## Environment Configuration

All functions respect environment variables:
- `LOG_MODE` - "console" or "file"
- `LOG_DIR` - Log directory path
- `LOG_FILE_NAME` - Log file name
- `LOG_MAX_SIZE_MB` - Max log file size
- `LOG_MAX_FILES` - Max backup files
- `LOG_MAX_AGE_DAYS` - Max retention days

## Real-World Example: Telemetry Gateway Initialization

### Before (Deprecated)

```go
import (
    "github.com/go-logr/zerologr"
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

func initializeOTel(ctx context.Context, tracerName, version string) (func(), error) {
    // Create logger and wrap in pointer
    logr := zerologr.New(logger.NewZeroLogger())

    // Confusing API with pointer parameter
    ctx, ctxLogger := instrumentation.GetLoggerForContext(ctx, &logr, tracerName)

    // Setup OTel with logger
    shutdownFn, err := instrumentation.SetupOTelSDK(ctx, tracerName, version, ctxLogger)
    if err != nil {
        return nil, err
    }

    return shutdownFn, nil
}
```

### After (Recommended)

```go
import (
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

func initializeOTel(ctx context.Context, tracerName, version string) (func(), error) {
    // Create logger with explicit options (overrideable via LOG_* env vars)
    logr := logger.CreateLogger(
        logger.WithConsoleSink(),
        logger.WithInfoLevel(),
    )
    ctx = logger.ContextWithLogr(ctx, logr)

    // Setup OTel with options
    // OTEL_EXPORTER_OTLP_ENDPOINT always takes precedence if set,
    // regardless of whether you use WithDefaultOTLPEndpoint or not
    shutdownFn, err := instrumentation.SetupOTelSDKWithOptions(
        ctx, tracerName, version, logr,
        instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
    )
    if err != nil {
        return nil, err
    }

    return shutdownFn, nil
}
```

**Key improvements:**
1. Logger package owns all logger operations
2. No pointer anti-pattern
3. Clear separation: create → store → use
4. No redundant context retrieval
5. Environment-aware defaults (respects LOG_* and OTEL_* env vars)
6. Better package boundaries
7. Easier to test and understand
8. Production-ready configuration with sensible defaults

## Behavioral Differences & Breaking Changes

Understanding these differences will help you avoid surprises during migration.

### ⚠️ Behavioral Change: No Direct `zerologr.New()` Wrapping Needed

**Old pattern:**
```go
zlog := logger.NewZeroLogger()
logr := zerologr.New(&zlog)  // Manual wrapping required
```

**New pattern:**
```go
logr := logger.CreateLogger()  // Returns logr.Logger already wrapped
```

**Why this changed:**
- `CreateLogger()` internally creates and wraps the zerolog logger
- Returns `logr.Logger` interface ready to use
- Eliminates boilerplate and pointer confusion

**What you lose:**
- Direct access to underlying `*zerolog.Logger` methods (`.Str()`, `.Int()`, etc.)
- Ability to use zerolog's builder pattern directly

**What you gain:**
- Standard `logr.Logger` interface compatible with all logr-aware libraries
- No pointer anti-patterns
- Cleaner, more idiomatic Go code

---

### ⚠️ Behavioral Change: Explicit Context Storage

**Old behavior:**
```go
// GetLoggerForContext automatically stored logger in context
ctx, logger := GetLoggerForContext(ctx, nil, "name")
// ctx now has logger stored (hidden behavior)
```

**New behavior:**
```go
// Explicit two-step process
logr := logger.CreateLogger()
ctx = logger.ContextWithLogr(ctx, logr)  // Explicit storage
```

**Why this is better:**
- No hidden side effects
- Clear code flow: create → store → use
- Easier to understand what's happening
- More testable (you control when/where logger is stored)

---

### ✅ Non-Breaking: Environment Variables Still Work

**Both old and new APIs respect environment variables:**
- `LOG_MODE=file` - Switch between console and file output
- `LOG_LEVEL=0` - Set log level (trace=-1, debug=0, info=1, warn=2, error=3)
- `LOG_FORMAT=raw` - Change output format
- And all other `LOG_*` variables

**Example:**
```bash
# Your code
logr := logger.CreateLogger(logger.WithInfoLevel())

# User can override at runtime
LOG_LEVEL=-1 ./your-app  # Now uses trace level despite WithInfoLevel()
```

This is intentional: code provides defaults, environment overrides.

---

### ✅ Non-Breaking: Multiple Retrieval Strategies

**New API provides three retrieval functions** with different error handling:

```go
// 1. Graceful error handling (use when logger existence is uncertain)
logger, err := logger.LogrFromContext(ctx)
if err != nil {
    return fmt.Errorf("logger required but not in context: %w", err)
}

// 2. Panic on missing logger (use when logger is required)
logger := logger.MustLogrFromContext(ctx)  // Panics if missing

// 3. Automatic fallback (use when logger is optional)
logger := logger.LogrFromContextOrDefault(ctx)  // Never fails, creates default
```

**Real-world examples showing when to use each:**

#### Example 1: Library function that requires logger

Use `MustLogrFromContext` when logger is required and its absence is a bug:

```go
func ProcessData(ctx context.Context, data []byte) error {
    // Fail fast - caller must provide logger
    logger := logger.MustLogrFromContext(ctx)
    logger.Info("Processing data", "size", len(data))

    // ... process data
    return nil
}
```

**Why**: Application code should have set up logger during startup. If it's missing, that's a programming error that should fail fast.

#### Example 2: HTTP middleware that works with or without logger

Use `LogrFromContextOrDefault` when logger is optional but you want to log anyway:

```go
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Graceful fallback - works either way
        logger := logger.LogrFromContextOrDefault(r.Context())
        logger.V(1).Info("Metrics recorded", "endpoint", r.URL.Path)
        next.ServeHTTP(w, r)
    })
}
```

**Why**: Middleware might be used in applications that don't set up logging. Creating a default logger ensures middleware still works.

#### Example 3: Library function with optional logging

Use `LogrFromContext` when you want to check if logger exists without creating one:

```go
func OptionalLogging(ctx context.Context) {
    // Check if logger exists, don't create if missing
    logger, err := logger.LogrFromContext(ctx)
    if err != nil {
        // Skip logging - caller didn't set up logger
        return
    }
    logger.Info("Optional logging enabled")
}
```

**Why**: Library code shouldn't force logger creation on callers who don't want logging. Check and skip if not present.

#### Example 4: Test helper that might run independently

Use `LogrFromContextOrDefault` in test utilities:

```go
func setupTestEnvironment(ctx context.Context) *TestEnv {
    // Safe: works in tests with or without logger setup
    logger := logger.LogrFromContextOrDefault(ctx)
    logger.Info("Setting up test environment")

    // ... setup test env
    return &TestEnv{}
}
```

**Why**: Test utilities might be called from different test contexts. Some tests set up loggers, some don't. This works either way.

**Decision guide:**

| Scenario | Use | Reason |
|----------|-----|--------|
| Application code that REQUIRES logger | `MustLogrFromContext` | Fail fast - missing logger is a bug |
| Middleware/utilities that work either way | `LogrFromContextOrDefault` | Graceful fallback - always works |
| Library code with optional logging | `LogrFromContext` | Check without creating - respect caller's choice |
| Test utilities | `LogrFromContextOrDefault` | Flexible - works in any test context |

---

### 📖 Summary of What Changed

| Aspect | Old API | New API | Breaking? |
|--------|---------|---------|-----------|
| Logger wrapping | Manual `zerologr.New()` | Automatic in `CreateLogger()` | ⚠️ Behavioral |
| Context storage | Implicit in `GetLoggerForContext` | Explicit `ContextWithLogr()` | ⚠️ Behavioral |
| Environment variables | Supported | Supported (same behavior) | ✅ Non-breaking |
| Error handling | Single approach | Three retrieval strategies | ✅ Non-breaking |
| Configuration API | Config structs | Functional options (`With*`) | ⚠️ Behavioral |

**Key takeaway:** All changes make the code more explicit and testable. There are no breaking changes - the new API is a straightforward upgrade.

## See Also

- [Logger Configuration API](logger-configuration-api.md) - Complete configuration documentation
- [examples/logger_initialization.go](../examples/logger_initialization.go) - 7 comprehensive examples