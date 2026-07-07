# External Integrations

**Analysis Date:** 2026-07-07

## APIs & External Services

**OpenTelemetry (OTLP):**
- OTLP gRPC Collector - Where traces are exported for distributed tracing
  - SDK/Client: `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
  - Configuration: `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable (e.g., `http://localhost:4317`)
  - Pattern: Optional - traces gracefully degrade if collector unavailable
  - Timeout: 5 seconds for initial connection (OTLPExporterTimeout)
  - Batch flush: 1 second intervals (OTLPBatchTimeout defined in `instrumentation/otel.go:204`)

**Trace Propagation:**
- W3C Trace Context (TraceContext) - Standard trace ID/span ID propagation across services
- W3C Baggage - Context propagation for baggage items
- Implementation: `instrumentation/otel.go:440-445` (newPropagator function)

## Data Storage

**Databases:**
- None - This is a library package with no database integration

**File Storage:**
- Local filesystem only (via lumberjack for log rotation)
  - Configuration: `LOG_DIR` environment variable (default: `/var/log`)
  - File name: `LOG_FILE_NAME` environment variable (default: `app.log`)
  - No cloud storage integration

**Caching:**
- In-memory tracer cache with provider change detection
  - Located: `instrumentation/tracer.go`
  - Smart caching for performance with automatic provider swap detection
  - No distributed cache, no external cache services

## Authentication & Identity

**Auth Provider:**
- None - Internal library, no user authentication required
- gRPC mTLS support available (optional, configurable in exporter setup)
  - TLS auto-detection: `instrumentation/otel.go:461-464` checks for `https://` in endpoint
  - TLS credentials: `google.golang.org/grpc/credentials`

## Monitoring & Observability

**Error Tracking:**
- None configured by default
- Library provides SetError() and Error() methods on SpanLogger for error marking
  - SetError: Marks span as failed and logs error
  - Error: Logs error but span succeeds
  - Example: `examples/basic/main.go:130`

**Logs:**
- Structured JSON via zerolog to configured sink (console or file)
- Log level: Configurable via `LOG_LEVEL` environment variable (-1=trace, 0=debug, 1=info, 2=warn, 3=error, 4=fatal)
- Format options: JSON (default), Raw, Plain - via `LOG_FORMAT` environment variable
- Caller information: Optional via `LOG_CALLER_DIR_LVL` environment variable

**Distributed Tracing:**
- OpenTelemetry SDK with OTLP gRPC exporter
- Trace IDs and span IDs automatically included in logs
- Automatic parent-child span relationships via context propagation
- Testing: In-memory exporter available via `instrumentation/oteltest` package

## CI/CD & Deployment

**Hosting:**
- GitHub-hosted repository (github.com/weka/go-weka-observability)
- Package registry: go.pkg.dev (Go package discovery)
- Library import: `go get github.com/weka/go-weka-observability`

**CI Pipeline:**
- GitHub Actions
  - Lint job: golangci-lint v2.8.0, Go vet, file name linting
  - Test job: go test with race detector, coverage reporting
  - Code coverage: Coverage artifact upload and comment on PRs
  - Triggers: Pull requests and pushes to main branch
  - Jobs defined in `.github/workflows/ci.yaml`

**Release Pipeline:**
- Git tag-based releases via GitHub Actions
  - Workflow: `.github/workflows/tag-release.yaml`
  - Creates releases from git tags

## Environment Configuration

**Required env vars (for trace export):**
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP collector endpoint (e.g., `http://localhost:4317`)
  - If not set: Traces not exported (graceful degradation)
  - Overrides: Code-configured endpoint via `WithDefaultOTLPEndpoint()`

**Optional env vars (logging):**
- `LOG_MODE` - Output mode: `console` (default) or `file`
- `LOG_LEVEL` - Minimum log level: -1 (trace), 0 (debug), 1 (info), 2 (warn), 3 (error), 4 (fatal)
- `LOG_FORMAT` - Format: `json` (default), `raw`, or `plain`
- `LOG_DIR` - Log directory for file mode (default: `/var/log`)
- `LOG_FILE_NAME` - Log filename for file mode (default: `app.log`)
- `LOG_MAX_SIZE_MB` - Max file size before rotation (default: 100)
- `LOG_MAX_FILES` - Max backup files to keep (default: 5)
- `LOG_MAX_AGE_DAYS` - Max days to retain logs (default: 28)
- `LOG_CALLER_DIR_LVL` - Caller directory nesting level (default: -1 disabled)
- `LOG_TIME_ONLY` - Use time-only format instead of full timestamp

**Optional env vars (OpenTelemetry):**
- `OTEL_GRPC_DISABLE_SERVICE_CONFIG` - Disable gRPC service config lookup (default: true to prevent DNS delays)
  - Set to `false` to enable gRPC service config resolution

**Secrets location:**
- No secrets stored in code
- No API keys required
- Network credentials (if using mTLS): Managed outside this library

## Webhooks & Callbacks

**Incoming:**
- None - This is a library package, not a server

**Outgoing:**
- Trace export to OTLP endpoint (async, batched every 1 second)
  - Endpoint: Configurable via `OTEL_EXPORTER_OTLP_ENDPOINT`
  - Protocol: gRPC
  - Batch size: Configurable in tracesdk.WithBatcher()
  - No retries on export failure (graceful degradation)

## Integration Patterns

**Logger Context Propagation:**
- `logger.ContextWithLogr(ctx, logger)` - Stores logger in context
- `logger.LogrFromContextOrDefault(ctx)` - Retrieves logger or uses default
- Location: `logger/context.go`

**Tracer Context Propagation:**
- `instrumentation.CreateLogSpan(ctx, name, keys..., values...)` - Creates owned span with tracer from context
- `instrumentation.CurrentSpanLogger(ctx)` - Borrows current span (SpanLoggerView, no End() method)
- `instrumentation.ContextWithTracer(ctx, tracer)` - Injects tracer into context for testing
- Location: `instrumentation/tracer_context.go`, `instrumentation/spanlogger.go`

**HTTP Tracing:**
- OTEL HTTP instrumentation via `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
- Example: `examples/http_tracing/` demonstrates client and server tracing
- Automatic trace propagation: W3C Trace Context headers in HTTP requests

**Test Isolation:**
- `instrumentation/oteltest.SetupTester(ctx)` - Context-based test isolation (parallel safe)
- `instrumentation/oteltest.SetupTesterWithProvider(ctx)` - Provider-based isolation (sequential only)
- Location: `instrumentation/oteltest/oteltest.go`

---

*Integration audit: 2026-07-07*
