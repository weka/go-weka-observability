# go-weka-observability

This library provides observability tools for Go applications, including a logger and OpenTelemetry (otel) instrumentation and tools.

## Features

- **Logger**: A robust logging tool with file rotation, environment-aware configuration, and context management
- **Otel Instrumentation**: Tools for instrumenting your Go applications with OpenTelemetry

## Getting Started

### Prerequisites

- Go version 1.22.2 or higher

### Installation

```bash
go get github.com/weka/go-weka-observability
```

## Quick Start

### Logger

```go
import "github.com/weka/go-weka-observability/logger"

// Create logger with environment defaults
logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverride())

// Store in context
ctx = logger.ContextWithLogr(ctx, logr)

// Use logger
logr.Info("Application started")
```

### Instrumentation

```go
import (
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

// Initialize logger
logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverride())
ctx = logger.ContextWithLogr(ctx, logr)

// Setup OpenTelemetry
shutdownFn, err := instrumentation.SetupOTelSDK(ctx, "my-service", "1.0.0", logr)
if err != nil {
    // Handle error
}
defer shutdownFn(ctx)

// Create traced operations
ctx, spanLogger, shutdown := instrumentation.GetLogSpan(ctx, "operation-name")
defer shutdown()

spanLogger.Info("Operation in progress")
```

## Documentation

- **[Logger Configuration API](docs/logger-configuration-api.md)** - Complete guide to logger configuration, modes, and environment variables
- **[Logger Initialization Migration Guide](docs/logger-initialization-migration.md)** - How to migrate from deprecated APIs to the new logger package
- **[Logger Initialization Examples](examples/logger_initialization.go)** - 7 comprehensive examples showing different initialization patterns

## Environment Variables

The logger respects the following environment variables:

- `LOG_MODE` - Output mode: `console` (default) or `file`
- `LOG_DIR` - Log directory for file mode (default: `/var/log`)
- `LOG_FILE_NAME` - Log file name for file mode
- `LOG_MAX_SIZE_MB` - Max log file size before rotation (default: 100)
- `LOG_MAX_FILES` - Max number of backup files (default: 5)
- `LOG_MAX_AGE_DAYS` - Max age for log retention (default: 28)
- `LOG_LEVEL` - Log level (default: info)
- `LOG_FORMAT` - Output format: `json` (default) or `raw`

## Legacy API Migration

If you're using the deprecated `instrumentation.GetLoggerForContext` API, please see the [Migration Guide](docs/logger-initialization-migration.md) for upgrade instructions