# Codebase Structure

**Analysis Date:** 2026-07-07

## Directory Layout

```
go-weka-observability/
├── logger/                    # Structured logging with file rotation
│   ├── logger.go             # Logger types and creation functions
│   ├── config.go             # Config structs and defaults
│   ├── options.go            # Functional options API
│   ├── context.go            # Context-based logger propagation
│   ├── writer.go             # Multi-level writers and rotation setup
│   ├── output_mode.go        # Output mode enum (console/file)
│   ├── log_format.go         # Log format enum (json/raw/plain)
│   ├── logger_test.go        # Logger tests
│   └── writer_test.go        # Writer tests
│
├── instrumentation/           # OpenTelemetry tracing + SpanLogger API
│   ├── spanlogger.go         # SpanLogger/SpanLoggerView types
│   ├── logspan.go            # Span creation helpers (client/server/producer/consumer)
│   ├── tracer.go             # Thread-safe tracer caching
│   ├── tracer_context.go     # Context-based tracer injection
│   ├── context.go            # Helper functions for context management
│   ├── otel.go               # OTel SDK setup and shutdown
│   ├── otel_config.go        # OTel configuration structs
│   ├── deprecations.go       # Deprecated API (avoid using)
│   ├── *_test.go             # All test files
│   │
│   └── oteltest/             # Test helpers for isolated testing
│       ├── tester.go         # SetupTester for parallel test isolation
│       ├── tester_test.go    # Tester tests
│       └── recorder.go       # Span recording for assertions
│
├── internal/                  # Internal utilities
│   └── version/              # Automatic version resolution
│       ├── version.go        # Module version and instrumentation scope
│       └── version_test.go   # Version tests
│
├── examples/                  # Usage examples
│   ├── basic/                # Basic logging + tracing example
│   │   └── main.go          # Demonstrates CreateLogSpan, helper functions
│   ├── http_tracing/        # HTTP server tracing example
│   ├── error_patterns/      # Error handling patterns
│   ├── span_lifecycle/      # Span creation and closure patterns
│   └── type_safe_spans/     # SpanLogger vs SpanLoggerView patterns
│
├── docs/                      # Documentation
│   ├── index.md              # Repo map
│   ├── logger-*.md           # Logger API documentation
│   ├── instrumentation-*.md  # Instrumentation API documentation
│   ├── spanlogger-api.md     # SpanLogger API reference
│   ├── adr/                  # Architecture Decision Records
│   ├── agents/               # Agent documentation
│   └── plans/                # Implementation plans
│
├── .planning/                 # GSD task planning
│   └── codebase/             # Codebase analysis documents (this file, ARCHITECTURE.md, etc.)
│
├── test-logs/                # Test fixture directory
├── test-logs-lumberjack/     # Lumberjack rotation test fixtures
│
├── .claude/                  # Claude Code configuration
├── .github/                  # GitHub workflow files
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
├── Taskfile.yaml             # Task runner commands
├── CLAUDE.md                 # Project instructions for Claude Code
├── CONTEXT.md                # Additional context
├── README.md                 # Project overview
├── .golangci.yaml            # Linter configuration
└── .ls-lint.yml              # File naming lint rules
```

## Directory Purposes

**logger/:**
- Purpose: Structured JSON logging with automatic file rotation
- Contains: Config management, functional options, multi-level writers, context propagation
- Key files: `logger.go` (main types), `config.go` (configuration), `writer.go` (multi-level writer setup)
- Related: `examples/basic/logger_initialization.go` shows usage

**instrumentation/:**
- Purpose: OpenTelemetry SDK integration with SpanLogger API for combined logging+tracing
- Contains: Span creation, tracer management, OTel SDK setup, test helpers
- Key files: `spanlogger.go` (owned/borrowed span types), `otel.go` (SDK setup), `tracer.go` (caching)
- Subpackage: `oteltest/` provides test isolation helpers

**instrumentation/oteltest/:**
- Purpose: Test helpers for isolated OTel testing
- Contains: In-memory span exporter, recorder for assertions
- Key files: `tester.go` (SetupTester function), `recorder.go` (span recording)
- Usage: Enables parallel test execution without interference

**internal/version/:**
- Purpose: Automatic module version and instrumentation scope resolution
- Contains: Version detection logic, instrumentation name resolution
- Key files: `version.go` (main logic)
- Exported functions: `GetInstrumentationVersion()`

**examples/:**
- Purpose: Demonstrate library usage patterns
- Contains: Basic example, HTTP tracing, error handling, span lifecycle
- Key files: `basic/main.go` (complete end-to-end example)
- Entry point: `basic/main.go:26` shows logger + OTel SDK setup pattern

**docs/:**
- Purpose: API documentation, architecture decisions, implementation guides
- Contains: Logger API, instrumentation API, decision records
- Key index: `docs/index.md` - navigation for all documentation

## Key File Locations

**Entry Points:**
- `logger/context.go:197` - `CreateLogger()` - main logger creation API
- `instrumentation/spanlogger.go:277` - `CreateLogSpan()` - span creation API
- `instrumentation/spanlogger.go:335` - `CreateRootLogSpan()` - root span creation
- `instrumentation/otel.go:SetupOTelSDKWithOptions()` - SDK initialization

**Configuration:**
- `logger/config.go` - Config, SinkConfig, FormatConfig structs
- `logger/options.go` - Functional option functions
- `instrumentation/otel_config.go` - OTelConfig struct and options

**Core Logic:**
- `logger/writer.go` - Multi-level writer setup, rotation via lumberjack
- `instrumentation/spanlogger.go` - SpanLogger and SpanLoggerView types
- `instrumentation/tracer.go` - Thread-safe tracer caching with provider change detection
- `instrumentation/context.go` - Context helpers for span and logger retrieval

**Testing:**
- `logger/logger_test.go` - Logger type tests
- `logger/writer_test.go` - Writer tests
- `instrumentation/oteltest/tester.go` - SetupTester for test isolation
- `instrumentation/oteltest/recorder.go` - Span recording for assertions

**Examples:**
- `examples/basic/main.go` - Complete end-to-end example with nested spans
- `examples/basic/logger_initialization.go` - Logger creation patterns
- `examples/http_tracing/main.go` - HTTP server tracing
- `examples/error_patterns/main.go` - Error handling (Error vs SetError)

## Naming Conventions

**Files:**
- Package files named after their primary type: `spanlogger.go` (SpanLogger type), `tracer.go` (tracer functions)
- Test files: `*_test.go` with `*_test` package for public API testing
- Examples: Under `examples/[category]/` with descriptive names

**Directories:**
- Package directories: lowercase, single word: `logger`, `instrumentation`
- Subpackages: lowercase: `oteltest`
- Feature directories under examples: descriptive: `http_tracing`, `error_patterns`, `span_lifecycle`

**Functions:**
- Public functions: CamelCase, verbs first: `CreateLogger()`, `CreateLogSpan()`, `SetupOTelSDKWithOptions()`
- Options: Prefix with `With`: `WithFileSink()`, `WithDebugLevel()`, `WithResourceAttributes()`
- Getters: Prefix with `Get` or context-aware: `GetTracer()`, `CurrentSpanLogger()`, `LogrFromContext()`
- Helpers: Lowercase `getOrCreateLogger()`, `enrichLogger()`, `addTraceIDsIfValid()`

**Types:**
- Config types: Suffix with Config: `Config`, `SinkConfig`, `FormatConfig`, `OTelConfig`
- Option types: `LoggerOption`, `OTelOption`
- Main types: PascalCase: `Logger`, `SpanLogger`, `SpanLoggerView`
- Enums: PascalCase: `OutputMode`, `LogFormat`, `LogLevel`

**Constants:**
- Mode/format constants: Descriptive: `ConsoleMode`, `FileMode`, `LogFormatJSON`, `LogFormatRaw`
- Sentinel errors: Prefix with `Err`: `ErrLoggerNotFound`, `ErrLogLevelOutOfBounds`
- Configuration defaults: Prefix with `default`: `defaultMaxSizeMB`, `defaultMaxFiles`

## Where to Add New Code

**New Feature - Logger Enhancement (e.g., new sink type):**
- Primary code: `logger/logger.go` (create new function), `logger/writer.go` (add writer logic)
- Configuration: `logger/config.go` (add config field), `logger/options.go` (add option function)
- Tests: `logger/logger_test.go` (new test), `logger/writer_test.go` (if writer-related)
- Example: `examples/basic/logger_initialization.go` (add usage example)

**New Feature - Instrumentation/Tracing:**
- Primary code: `instrumentation/spanlogger.go` (for span creation), `instrumentation/tracer.go` (for tracer changes)
- Configuration: `instrumentation/otel_config.go` (if new config needed), `instrumentation/otel.go` (if SDK setup change)
- Tests: `instrumentation/*_test.go` (test files exist for each module)
- Test helpers: `instrumentation/oteltest/tester.go` (for test isolation features)
- Example: `examples/span_lifecycle/main.go` or appropriate example directory

**New Example:**
- Create directory under `examples/[feature-name]/`
- Main entry point: `examples/[feature-name]/main.go`
- Related code: Additional `.go` files in same directory
- Pattern: Follow `examples/basic/main.go` structure

**New Utility/Internal Package:**
- Location: `internal/[feature]/`
- Keep internal - not part of public API
- Tests: `internal/[feature]/*_test.go`
- Example: `internal/version/` for version management

**Documentation:**
- API documentation: `docs/[topic]-api.md` (paired with corresponding code)
- Architecture decisions: `docs/adr/[number]-[title].md`
- Implementation guides: `docs/guides/[topic].md`
- Examples: `examples/[topic]/` with runnable code

## Special Directories

**examples/:**
- Purpose: Runnable examples demonstrating library usage
- Generated: No
- Committed: Yes
- Running: `go run ./examples/basic/` or `go run ./examples/http_tracing/`
- Importance: Documentation through working code

**test-logs/ and test-logs-lumberjack/:**
- Purpose: Test fixture directories for logger rotation tests
- Generated: Yes (by tests at runtime)
- Committed: No (in .gitignore)
- Contents: Temporary log files created during test runs

**docs/adr/:**
- Purpose: Architecture Decision Records
- Contains: Numbered ADRs documenting design choices
- Format: Markdown with Context/Decision/Consequences structure

**.planning/codebase/:**
- Purpose: GSD task planning documents (ARCHITECTURE.md, STRUCTURE.md, etc.)
- Generated: By /gsd-map-codebase agent
- Committed: Yes
- Contents: Analysis documents for use by /gsd-plan-phase and /gsd-execute-phase

---

*Structure analysis: 2026-07-07*
