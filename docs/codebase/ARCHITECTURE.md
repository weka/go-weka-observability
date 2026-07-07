<!-- refreshed: 2026-07-07 -->
# Architecture

**Analysis Date:** 2026-07-07

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│            Application Layer (Users/Examples)               │
│              `examples/` - Usage patterns                    │
└────────────┬──────────────────────────────────────┬──────────┘
             │                                      │
             ▼                                      ▼
┌──────────────────────────────┐    ┌───────────────────────────┐
│   Logger Package             │    │  Instrumentation Package  │
│  `logger/` - Structured      │    │  `instrumentation/` -     │
│   Logging with Rotation      │    │   OpenTelemetry Tracing   │
│                              │    │   + SpanLogger API        │
│  - Config management         │    │                           │
│  - Multi-level writers       │    │  - SpanLogger/View        │
│  - Context propagation       │    │  - Tracer management      │
│  - File rotation (lumberjack)│    │  - SDK setup              │
│  - Environment overrides     │    │  - Test helpers           │
└──────────────────────────────┘    └───────────────────────────┘
             │                                      │
             └──────────────┬───────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                       │
├─────────────────────────────────────────────────────────────┤
│  External Dependencies:                                     │
│  - zerolog (JSON structured logging)                       │
│  - lumberjack (file rotation)                              │
│  - go-logr (structured logger interface)                   │
│  - OpenTelemetry SDK (tracing)                             │
│  - OTLP gRPC exporter (trace export)                       │
│  - kelseyhightower/envconfig (env parsing)                 │
└─────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| **Logger** | Structured JSON logging with file rotation and multi-level output | `logger/logger.go`, `logger/writer.go` |
| **LoggerConfig** | Configuration structs (SinkConfig, FormatConfig) with env overrides | `logger/config.go` |
| **LoggerOptions** | Functional options API for declarative configuration | `logger/options.go` |
| **LoggerContext** | Context-based logger propagation and retrieval | `logger/context.go` |
| **SpanLogger** | Owned span with compile-time safety (has End() method) | `instrumentation/spanlogger.go` |
| **SpanLoggerView** | Borrowed span without End() (compile-time read-only) | `instrumentation/spanlogger.go` |
| **Tracer** | Thread-safe tracer caching with provider change detection | `instrumentation/tracer.go` |
| **OTelConfig** | OTel SDK configuration with env overrides | `instrumentation/otel_config.go` |
| **OTelSDK** | OpenTelemetry SDK setup and shutdown | `instrumentation/otel.go` |
| **TestHelpers** | Isolated in-memory span exporter for test parallelization | `instrumentation/oteltest/` |
| **Versioning** | Automatic module version and instrumentation scope resolution | `internal/version/version.go` |

## Pattern Overview

**Overall:** Two-Package Observability Toolkit with Ownership-Based Type Safety

**Key Characteristics:**
- **Functional Options**: Both logger and instrumentation use functional option pattern for clean API
- **Environment-First Configuration**: 12-factor app compliance - env vars always override code defaults
- **Context-Based Propagation**: Both logger and tracer use context.Context for lifecycle management
- **Compile-Time Safety**: SpanLogger (owned) vs SpanLoggerView (borrowed) prevents resource leaks at compile time
- **Integration-Style Testing**: Real implementations (servers, temp files) instead of mocks
- **Zero-Allocation JSON Logging**: zerolog + lumberjack for production-grade performance

## Layers

**Application Layer:**
- Purpose: User code consuming the observability toolkit
- Location: `examples/`
- Contains: HTTP tracing, error patterns, span lifecycle examples
- Depends on: logger and instrumentation packages

**Logger Package:**
- Purpose: Production-grade structured logging with automatic file rotation
- Location: `logger/`
- Contains: Config structs, functional options, multi-level writers, context propagation
- Depends on: zerolog, lumberjack, go-logr
- Used by: Instrumentation package, user code, OpenTelemetry SDK setup

**Instrumentation Package:**
- Purpose: OpenTelemetry tracing integration with combined logging+tracing via SpanLogger
- Location: `instrumentation/`
- Contains: SpanLogger API, tracer management, OTel SDK setup, test helpers
- Depends on: logger package, OpenTelemetry SDK and exporters
- Used by: User code for creating and managing traced operations

**Infrastructure Layer:**
- Purpose: External dependencies providing core functionality
- Contains: zerolog, lumberjack, OpenTelemetry SDK, gRPC exporter
- Used by: All higher layers

## Data Flow

### Primary Request Path (SpanLogger Lifecycle)

1. **Initialization** (`examples/basic/main.go:26-92`)
   - `logger.CreateLogger()` with functional options → Creates logr.Logger
   - `logger.ContextWithLogr(ctx, logr)` → Stores logger in context
   - `instrumentation.SetupOTelSDKWithOptions()` → Initializes OTel SDK, sets global tracer provider

2. **Span Creation** (`instrumentation/spanlogger.go:277-301` - CreateLogSpan)
   - Caller invokes `instrumentation.CreateLogSpan(ctx, "operation_name", key, value)`
   - `getOrCreateLogger(ctx)` → Retrieves logger from context or creates default
   - `enrichLogger()` → Adds operation name and key-value pairs to logger
   - `createChildSpan()` → Creates child span from current context span
   - `addTraceIDsIfValid()` → Enriches logger with trace_id and span_id from span
   - `createSpanShutdownFunc()` → Wraps span.End() with logging
   - Returns `(ctx, *SpanLogger)` with both owned span and enriched logger

3. **Logging Within Span** (`instrumentation/spanlogger.go:78-125`)
   - Caller invokes `logger.Info(msg, key, value)`
   - Delegates to underlying logr.Logger (adds to log output)
   - Calls `span.SetAttributes()` (adds to span attributes in OTel)
   - Calls `span.AddEvent()` (creates span event in OTel)
   - Result: Log lines include trace_id/span_id, span has event with attributes

4. **Span Closure** (`instrumentation/spanlogger.go:165-167`)
   - Caller invokes `defer logger.End()`
   - Calls `shutdownFunc()` which calls `span.End()` (closes OTel span)
   - Span is exported to OTLP collector (if configured)

### Helper Function Pattern (SpanLoggerView)

1. **Parent Creates Span** (see above)
2. **Helper Borrows Current Span** (`instrumentation/spanlogger.go:233-245` - CurrentSpanLogger)
   - Helper invokes `view := instrumentation.CurrentSpanLogger(ctx)`
   - Retrieves current span from context using `trace.SpanFromContext(ctx)`
   - Creates SpanLoggerView (no End() method)
   - Helper logs via `view.Info()` - logs and span events added under parent's span
3. **Helper Returns** (parent still owns span)
   - Helper cannot call `view.End()` (compile error - method doesn't exist)
   - Parent's `defer logger.End()` properly closes span when outer function exits

### Configuration Override Flow

**Logger Configuration** (`logger/config.go:131-147` - NewConfigFromEnv):
1. Create Config from defaults or functional options
2. Call `envconfig.Process("LOG", &config.Sink)` → Applies LOG_MODE, LOG_DIR, etc.
3. Call `envconfig.Process("LOG", &config.Format)` → Applies LOG_LEVEL, LOG_FORMAT, etc.
4. Env vars always override code defaults (12-factor app pattern)

**OTel Configuration** (`instrumentation/otel_config.go` - NewOTelConfigFromEnv):
1. Create OTelConfig from defaults or functional options
2. Check `os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")` → Overrides code default
3. Env vars always take precedence

**State Management:**
- Logger: Immutable Config passed to CreateLoggerFrom, no mutable state
- Tracer: Global singleton cache with RWMutex protection, automatic invalidation on provider change
- Spans: Owned by SpanLogger (must call End()), context-based temporary storage

## Key Abstractions

**Config-Based Creation:**
- Purpose: Decouple configuration from logger/tracer creation
- Examples: `logger.Config`, `instrumentation.OTelConfig`
- Pattern: Create config with defaults, apply env overrides, pass to Create* function

**Functional Options:**
- Purpose: Provide clean API for setting defaults while preserving env override capability
- Examples: `logger.CreateLogger(logger.WithFileSink(), logger.WithInfoLevel())`
- Pattern: Options modify Config struct, then env vars override

**Owned vs Borrowed Spans:**
- Purpose: Compile-time safety for span lifecycle management
- `SpanLogger`: Has End() method, caller owns span, must defer End()
- `SpanLoggerView`: No End() method, caller borrows span, cannot close it
- Pattern: Type system enforces resource cleanup (no runtime panics or leaks)

**Tracer Caching:**
- Purpose: High-performance tracer access with automatic provider change detection
- Layers: Context override → Cache hit → Cache miss → Create from provider
- Pattern: RWMutex double-check locking, fast path is ~10-50ns read lock

**Multi-Level Writer:**
- Purpose: Route different log levels to separate files (info vs error)
- Example: `logger.SpecificLevelWriter` checks level before writing
- Pattern: Info and below → info.log, Warn and above → error.log

## Entry Points

**Logger Creation:**
- Location: `logger/context.go:197-204` - `CreateLogger()`
- Triggers: Application startup
- Responsibilities: Parse functional options, apply env overrides, return logr.Logger

**Span Creation:**
- Location: `instrumentation/spanlogger.go:277-301` - `CreateLogSpan()`
- Triggers: When entering operation that should be traced
- Responsibilities: Get/create logger, create child span, enrich logger with trace IDs, return owned SpanLogger

**OTel SDK Setup:**
- Location: `instrumentation/otel.go:SetupOTelSDKWithOptions()`
- Triggers: Application startup (typically in main())
- Responsibilities: Initialize SDK, set global provider, configure exporter, return shutdown function

**Helper Function Logging:**
- Location: `instrumentation/spanlogger.go:233-245` - `CurrentSpanLogger()`
- Triggers: Helper function needs to log under current span
- Responsibilities: Retrieve current span from context, return SpanLoggerView (compile-time safe)

## Architectural Constraints

- **Threading:** Go concurrency model - goroutines safe, no global mutable state except tracer cache (RWMutex protected)
- **Global state:** Tracer cache singleton in `instrumentation/tracer.go` with mutex protection, OpenTelemetry provider (set globally)
- **Circular imports:** None detected - logger is independent, instrumentation depends on logger
- **Context Propagation:** Both logger and span must be propagated explicitly via `context.Context`
- **Resource Management:** Spans MUST call End() (via defer), SDK shutdown must be deferred - enforced by SpanLogger ownership model
- **Environment Variables:** Always override code defaults (12-factor app pattern) - applied at Config creation time
- **Immutability:** Config structs are value types, logr.Logger is immutable interface, only tracer cache is mutable

## Anti-Patterns

### Creating Span But Forgetting to End It

**What happens:** Code creates span with `CreateLogSpan()` but doesn't defer `End()`
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
logger.Info("Processing")  // Compiles but span never closes!
```

**Why it's wrong:** Span remains open indefinitely, consuming memory, and never gets exported to collector

**Do this instead:** Always defer End() immediately after CreateLogSpan
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
defer logger.End()  // Guaranteed to call even on panic
logger.Info("Processing")
```

### Using SpanLogger in Helper Functions

**What happens:** Helper receives owned SpanLogger and could accidentally call End()
```go
func helper(ctx context.Context, logger *instrumentation.SpanLogger) {
    logger.Info("Doing work")
    logger.End()  // Oops! Closed parent's span early
}
```

**Why it's wrong:** Helper ends the span prematurely, child operations fail

**Do this instead:** Pass context to helper, it calls CurrentSpanLogger to borrow current span
```go
func helper(ctx context.Context) {
    view := instrumentation.CurrentSpanLogger(ctx)
    view.Info("Doing work")
    // view.End()  // Compile error - method doesn't exist!
}
```

### Hardcoding Configuration Instead of Using Env Overrides

**What happens:** Logger created with fixed config, environment variable ignored
```go
logr := logger.CreateLogger(
    logger.WithFileSink("/var/log", "app.log"),
    logger.WithInfoLevel(),
)
// User sets LOG_LEVEL=0 but it has no effect
```

**Why it's wrong:** Violates 12-factor app pattern, users cannot adjust logging without recompiling

**Do this instead:** Functional options set code defaults, env vars always override
```go
logr := logger.CreateLogger(
    logger.WithFileSink("/var/log", "app.log"),  // Code default
    logger.WithInfoLevel(),  // Code default
)
// Environment: export LOG_LEVEL=0  # Now takes precedence!
// User's env var overrides code default
```

### Mixing Owned and Borrowed Span Usage

**What happens:** Code doesn't distinguish between created spans and borrowed current spans
```go
ctx, logger1 := instrumentation.CreateLogSpan(ctx, "operation")
// ...later, assume there's a current span in ctx
logger2 := instrumentation.CurrentSpanLogger(ctx)  // Type mismatch conceptually
logger2.End()  // Wrong! This is SpanLoggerView, no End() method - compile error
```

**Why it's wrong:** Conceptual confusion about ownership, leads to resource leaks

**Do this instead:** Use owned spans when you create, use views when you borrow
```go
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
defer logger.End()  // Owned span - I created it, I end it

helper(ctx)  // Helper borrows current span

func helper(ctx context.Context) {
    view := instrumentation.CurrentSpanLogger(ctx)
    view.Info("Helper work")  // Borrowed - no End() method
}
```

## Error Handling

**Strategy:** Early return with wrapped errors, no panics except for invariant violations

**Patterns:**
- Logger configuration errors logged to slog, defaults used as fallback
- OTel SDK setup errors returned to caller for proper shutdown on failure
- Span key-value pair count validation with panic (invariant: even number of pairs)
- Tracer retrieval from context returns zero-value no-op tracer if not found (graceful degradation)

## Cross-Cutting Concerns

**Logging:** 
- Structured JSON via zerolog
- Multi-level writers separate info and error logs in file mode
- Caller information with configurable directory depth
- Integration with OpenTelemetry trace IDs via SpanLogger

**Validation:** 
- Configuration struct validation in functional options
- Panic on odd-numbered key-value pairs to CreateLogSpan
- Safe fallbacks for missing context values

**Authentication:** 
- Not applicable - this is an observability toolkit
- OTLP endpoint is configured, no auth implemented (can be added via gRPC interceptors)

---

*Architecture analysis: 2026-07-07*
