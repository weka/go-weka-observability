# go-weka-observability

Observability toolkit for Go applications: structured logging with automatic rotation + OpenTelemetry instrumentation.

**📖 [Complete Documentation](docs/logger-configuration-api.md)**

## Installation

```bash
go get github.com/weka/go-weka-observability
```

## Quick Start

### Console Logger (Default)

```go
import "github.com/weka/go-weka-observability/logger"

// Simple console logger - perfect for containers/K8s
logr := logger.CreateLogger()
logr.Info("Application started")
```

### File Logger with Rotation

```go
// File logger with automatic rotation
logr := logger.CreateLogger(
    logger.WithFileSink("/var/log", "app.log"),
    logger.WithRotation(100, 5, 28), // 100MB, 5 files, 28 days
)
```

### Environment-Aware Configuration

```go
// Respects LOG_MODE, LOG_DIR, LOG_FILE_NAME, etc.
logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())
ctx = logger.ContextWithLogr(ctx, logr)

// Use logger
logger := logger.MustLogrFromContext(ctx)
logger.Info("Operation started")
```

### With OpenTelemetry

```go
import (
    "github.com/weka/go-weka-observability/instrumentation"
    "github.com/weka/go-weka-observability/logger"
)

// Initialize logger and context
logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())
ctx = logger.ContextWithLogr(ctx, logr)

// Setup OpenTelemetry
shutdownFn, err := instrumentation.SetupOTelSDK(ctx, "my-service", "1.0.0", logr)
if err != nil {
    panic(err)
}
defer shutdownFn(ctx)

// Create traced operations with automatic logging
ctx, spanLogger, end := instrumentation.GetLogSpan(ctx, "operation-name")
defer end()

spanLogger.Info("Operation in progress", "key", "value")
```

## Documentation

- **[Logger Configuration API](docs/logger-configuration-api.md)** - Complete configuration guide, use cases, environment variables, best practices, troubleshooting
- **[Migration Guide](docs/logger-initialization-migration.md)** - Upgrade from deprecated `GetLoggerForContext` API
- **[Examples](examples/)** - Runnable code examples demonstrating common patterns

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_MODE` | `console` | Output mode: `console` or `file` |
| `LOG_DIR` | `/var/log` | Log directory (file mode only) |
| `LOG_FILE_NAME` | - | Log file name (file mode only) |
| `LOG_MAX_SIZE_MB` | `100` | Max file size before rotation (MB) |
| `LOG_MAX_FILES` | `5` | Max number of backup files |
| `LOG_MAX_AGE_DAYS` | `28` | Max age for log retention (days) |
| `LOG_LEVEL` | `info` | Minimum log level (trace/debug/info/warn/error) |
| `LOG_FORMAT` | `json` | Output format: `json`, `raw`, `plain` |
| `LOG_TIME_ONLY` | `false` | Use time-only format instead of full timestamp |
| `LOG_CALLER_DIR_LVL` | `-1` | Number of directory levels in caller field (-1=disabled) |

See [Configuration Guide](docs/logger-configuration-api.md#environment-configuration) for complete details.

## Features

- **Structured Logging**: Zero-allocation JSON logging via [zerolog](https://github.com/rs/zerolog)
- **Automatic Rotation**: File rotation with [lumberjack](https://github.com/natefinch/lumberjack)
- **Environment-Aware**: 12-factor app configuration via environment variables
- **Context Management**: Logger propagation through context
- **Flexible Configuration**: Functional options + struct-based config
- **OpenTelemetry Integration**: Automatic span creation with logger injection
- **Multi-Level Files**: Separate files for info and error logs
