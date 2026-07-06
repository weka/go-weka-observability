# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go observability toolkit providing structured logging with automatic rotation and OpenTelemetry instrumentation. Two main packages:

- **`logger`** - Zero-allocation JSON logging via zerolog with automatic file rotation via lumberjack
- **`instrumentation`** - OpenTelemetry tracing with SpanLogger API that combines logging and tracing

## Documentation
@docs/index.md

## Linter-Driven Development

This project uses the **go-linter-driven-development** workflow.

**IMPORTANT**: Before implementing any Go code, invoke the `go-linter-driven-development:linter-driven-development` skill.

## Quality Commands

### Test Command
```bash
task test
```

### Lint Command
```bash
task lintwithfix
```

## Commands

```bash
# Install task runner (if needed)
brew install go-task/tap/go-task

# Primary workflow - format, vet, lint with auto-fix
task lintwithfix

# Run tests
task test                    # With race detector
task test-coverage           # Generate coverage.html
go test -v -run TestName ./logger/...  # Single test

# Lint
task lint                    # Read-only
task lint-new-from-main      # Only changed files vs main (for PRs)

# Format
task fmt                     # All files
task fmt-changed             # Only changed files

# Setup pre-commit hooks
task setup-hooks
```

## Pre-commit Hooks

```bash
task setup-hooks    # Install hooks
```

Hooks run `task fmt` (formatters) and `task lintfix-no-output` (lint autofix) before commits.

## Architecture

### Logger Package (`logger/`)

Configuration flows through `Config` struct with functional options pattern:

```go
logr := logger.CreateLogger(
    logger.WithFileSink("/var/log", "app.log"),
    logger.WithRotation(100, 5, 28),
    logger.WithInfoLevel(),
)
ctx = logger.ContextWithLogr(ctx, logr)
```

Environment variables (LOG_MODE, LOG_LEVEL, LOG_FORMAT, etc.) always override code defaults following 12-factor app pattern.

Key files:
- `config.go` - Config structs (SinkConfig, FormatConfig, RotationConfig)
- `options.go` - Functional options (WithFileSink, WithRotation, etc.)
- `writer.go` - Multi-level writers, log rotation setup
- `context.go` - Context-based logger propagation

### Instrumentation Package (`instrumentation/`)

**SpanLogger API** - Type-safe span ownership with compile-time safety:

- `SpanLogger` - Owned span, MUST call `End()`
- `SpanLoggerView` - Borrowed span, cannot call `End()` (no method exists)

```go
// Create owned span - must defer End()
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation", "key", "value")
defer logger.End()

// Borrow current span - no End() method
view := instrumentation.CurrentSpanLogger(ctx)
view.Info("Helper logging")  // view.End() would be compile error
```

Convenience functions for span kinds: `CreateServerLogSpan`, `CreateClientLogSpan`, `CreateProducerLogSpan`, `CreateConsumerLogSpan`.

Key files:
- `spanlogger.go` - SpanLogger/SpanLoggerView types and creation functions
- `otel.go` - SetupOTelSDKWithOptions, SDK configuration
- `otel_config.go` - OTelConfig struct and options
- `tracer.go` - GetTracer, tracer management
- `deprecations.go` - Old API (GetLogSpan, GetLoggerForContext) - avoid using

### Testing Helpers (`instrumentation/oteltest/`)

Provides `SetupTester()` for test isolation with in-memory span exporter.

## Import Organization

Imports are grouped in this order (enforced by gci formatter):
1. Standard library
2. External packages
3. `github.com/weka/go-weka-observability` (this project)
4. `github.com/weka` (other weka packages)

## Deprecation Notes

Old API functions in `deprecations.go` are deprecated. Use instead:
- `GetLoggerForContext` → `logger.CreateLogger()` + `logger.ContextWithLogr()`
- `GetLogSpan` → `CreateLogSpan` / `CurrentSpanLogger` / `CreateRootLogSpan`
- `SetupOTelSDK` → `SetupOTelSDKWithOptions`
