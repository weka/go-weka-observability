# Logger Initialization Migration Guide

This document demonstrates how to migrate from the deprecated `GetLoggerForContext` API to the new, cleaner logger initialization pattern.

## Overview

The old API had confusing triple behavior based on nil pointer checks and unclear naming. The new API provides explicit, single-purpose functions that are easier to understand and use correctly.

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

## See Also

- [Logger Configuration API](logger-configuration-api.md) - Complete configuration documentation
- [examples/logger_initialization.go](../examples/logger_initialization.go) - 7 comprehensive examples