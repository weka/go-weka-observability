# Technology Stack

**Analysis Date:** 2026-07-07

## Languages

**Primary:**
- Go 1.25.0 - All production code and examples

**Secondary:**
- YAML - Configuration files (Taskfile.yaml, .golangci.yaml, CI/CD workflows)
- Markdown - Documentation

## Runtime

**Environment:**
- Go 1.25.0 (specified in `go.mod`)

**Package Manager:**
- Go modules
- Lockfile: Present (`go.sum`)

## Frameworks

**Core:**
- zerolog v1.34.0 - Zero-allocation JSON structured logging via `github.com/rs/zerolog`
- go-logr v1.4.3 - Abstract logging interface via `github.com/go-logr/logr`
- zerologr v1.2.3 - zerolog adapter implementing go-logr interface via `github.com/go-logr/zerologr`
- OpenTelemetry v1.42.0 - Distributed tracing instrumentation via `go.opentelemetry.io/otel`

**Trace Export:**
- OTLP gRPC Exporter v1.42.0 - Sends traces to OpenTelemetry collector via `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
- OpenTelemetry SDK v1.42.0 - Trace provider and SDK setup via `go.opentelemetry.io/otel/sdk`

**Log Rotation:**
- lumberjack v2.2.1 - Automatic log file rotation via `gopkg.in/natefinch/lumberjack.v2`

**HTTP Instrumentation:**
- OTEL HTTP Instrumentation v0.67.0 - Automatic HTTP client/server tracing via `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`

**Testing:**
- testify v1.11.1 - Assertion library and test utilities via `github.com/stretchr/testify`

**Build/Dev:**
- golangci-lint v2.8.0 - Multi-linter with strict code quality checks (via Taskfile)
- gci - Import formatting and organization (via Taskfile)
- goimports - Import optimization (via Taskfile)
- go-junit-report v2 - JUnit XML test reporting (GitHub Actions)
- ls-lint v2.3.1 - Filename and directory naming validation

**Utilities:**
- envconfig v1.4.0 - Environment variable parsing via `github.com/kelseyhightower/envconfig`
- gRPC v1.79.3 - RPC framework for OTLP trace export via `google.golang.org/grpc`

**Transitive Dependencies:**
- google.golang.org/protobuf v1.36.11 - Protocol buffers for gRPC messages
- google.golang.org/genproto - Google API definitions for gRPC
- golang.org/x/net - Network utilities
- golang.org/x/sys - System-specific utilities

## Configuration

**Environment:**
- Configuration via environment variables following 12-factor app pattern
- LOG_* prefix for logger configuration (LOG_MODE, LOG_LEVEL, LOG_FORMAT, LOG_DIR, LOG_FILE_NAME, LOG_MAX_SIZE_MB, LOG_MAX_FILES, LOG_MAX_AGE_DAYS, LOG_CALLER_DIR_LVL, LOG_TIME_ONLY)
- OTEL_* prefix for OpenTelemetry configuration (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_GRPC_DISABLE_SERVICE_CONFIG)

**Build:**
- `.golangci.yaml` - golangci-lint configuration in project root with comprehensive linter rules
- `Taskfile.yaml` - Task runner configuration for code quality and testing pipeline
- `.pre-commit-config.yaml` - Pre-commit hooks for automatic code formatting and linting
- `.ls-lint.yml` - File/directory naming validation rules
- `go.mod` / `go.sum` - Go module dependencies

## Platform Requirements

**Development:**
- Go 1.25.0 or later
- Task runner (installable via `brew install go-task/tap/go-task`)
- Pre-commit hooks (installable via `task setup-hooks`)

**Production:**
- No external dependencies for logger package (standalone library)
- Optional: OpenTelemetry collector endpoint at configurable address (default: localhost:4317 via OTEL_EXPORTER_OTLP_ENDPOINT)
- Filesystem write access for file-based logging (when using FileSink)

## Key Dependencies Analysis

**Critical (Core Functionality):**
- `zerolog` - Essential for zero-allocation structured logging
- `go-logr/logr` - Provides stable logging abstraction
- `OpenTelemetry SDK` - Required for trace instrumentation
- `lumberjack` - Handles automatic log rotation to prevent disk full scenarios

**Infrastructure:**
- `gRPC` - Enables trace export to OTLP collector
- `envconfig` - Enables 12-factor configuration pattern
- `otelhttp` - Provides automatic HTTP tracing without code changes

## Deployment Model

This is a **library package** meant to be imported into Go applications. It provides:
1. Standalone logger package with zero external dependencies
2. Instrumentation package with optional trace export (graceful degradation if collector unavailable)
3. No server, no database, no persistent state

Applications using this library are responsible for:
- Initializing logger and OpenTelemetry SDK in their main()
- Configuring OTEL_EXPORTER_OTLP_ENDPOINT if trace export is desired
- Providing filesystem access for file-based logging if needed

---

*Stack analysis: 2026-07-07*
