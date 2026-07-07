# Coding Conventions

**Analysis Date:** 2026-07-07

## Naming Patterns

**Packages:**
- Flatcase, lowercase only (e.g., `logger`, `instrumentation`)
- Avoid generic names (`utils`, `common`, `domain`)
- Avoid collisions with stdlib or common packages (use specific names like `wekatrace` instead of `trace`)
- Semantic, intention-driven names

**Types:**
- PascalCase (e.g., `SpanLogger`, `Config`, `SinkConfig`)
- Self-describing domain names (e.g., `TaskID` for task identifiers, not raw `string`)
- Type names should reflect intent and behavior, not just shape

**Functions:**
- camelCase (e.g., `CreateLogger`, `WithFileSink`, `GetStderrWriter`)
- Public API functions use full names without abbreviation
- Option functions follow pattern: `With<Feature>` (e.g., `WithFileSink`, `WithRotation`, `WithJSONFormat`)
- Constructor names: `New<Type>`, `Create<Type>`, or semantic names like `Parse<Type>`
- Helper functions in tests: descriptive names like `testLogConfig()`, `captureStdoutWithMultiLevelWriter()`

**Variables:**
- camelCase for local variables
- Descriptive names, avoid single letters except for loop counters
- Abbreviations acceptable for well-known names (e.g., `ctx` for context, `t` for *testing.T)

**Constants:**
- UPPER_SNAKE_CASE for package-level constants (e.g., `defaultMaxSizeMB`, `logLevelInfo`)
- Grouped logically, one constant per const block unless related
- Use const groups for related values

**Enums/Type aliases:**
- PascalCase (e.g., `OutputMode`, `LogFormat`)
- String-based constants with semantic names (e.g., `ConsoleMode = "console"`, `FileMode = "file"`)

## Code Style

**Formatting:**
- golangci-lint v2.8.0 (with formatters: goimports, gci, golines, gofmt, gofumpt)
- Line length: 120 characters max (golines)
- Indentation: 4-space tabs (configured in golines)
- Go version: Module-based (go.mod for dependency management)

**Linting:**
- Config file: `.golangci.yaml` (comprehensive v2 rules)
- Max cyclomatic complexity: 10 (cyclop)
- Max cognitive complexity: 15 (gocognit)
- Enforced linters: staticcheck, govet, gocritic, spancheck, errorlint, errcheck, godoclint, revive, testifylint
- Mandatory pre-commit hooks: `task fmt` and `task lintfix-no-output`

**Linter Enforcement:**
- All linter issues must be resolved
- No nolint directives without written explanation and specificity
- nolint rules require `//nolint:linter // explanation` format (no space before //)
- nolint requires specific linter name(s), never bare `//nolint`

## Import Organization

**Order (enforced by gci):**
1. Standard library
2. External packages (default)
3. `github.com/weka/go-weka-observability` (this project)
4. `github.com/weka` (other weka packages)

**Examples from codebase:**
```go
// Good
import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"

	"github.com/weka/go-weka-observability/logger"
	zerologger "github.com/weka/go-weka-observability/logger"
)
```

**Import aliases:**
- Use semantic aliases when import names collide with local types
- Example: `zerologger "github.com/weka/go-weka-observability/logger"` to avoid collision with local `logger` package

**Blank imports:**
- Document with `// <reason>` on the line before
- Use only when necessary (e.g., for side-effects, registering drivers)

## Error Handling

**Return patterns:**
- Never return nil values; return error instead
- OK to return `(val, nil)` or `(nil, err)` - the real value is one or the other
- Use custom error types for domain-specific errors
- Wrap errors with context using `fmt.Errorf("%w", err)` for error chaining

**Error creation:**
- Use `fmt.Errorf` for dynamic errors (allowed to use `//nolint:err113` when intentional)
- Use custom `Error` types for structured error handling
- Error type naming: `<Domain>Error` suffix (e.g., `ValidationError`)

**Nil checking:**
- Validate function arguments in constructors; no nil checks needed inside methods
- Use "defensive programming" sparingly - prefer constructor validation
- Never pass `nil` to functions; constructors should enforce non-nil preconditions

**Error logging:**
- Log errors with context: `log.Error().Err(err).Msg("operation failed")`
- Use structured logging with context fields
- Avoid generic error messages

## Logging

**Framework:** zerolog for structured logging, logr for OTel integration

**Configuration patterns:**
- Use functional options: `logger.CreateLogger(logger.WithFileSink(...), logger.WithRotation(...))`
- Environment variables override code (12-factor app pattern)
- Default configuration via `logger.DefaultConfig()`
- Env overrides via `logger.NewConfigFromEnv(defaultConfig)`

**Logging levels:**
- Trace (-1): Extremely detailed debugging
- Debug (0): Development debugging
- Info (1): Normal operational messages (default level)
- Warn (2): Warning conditions
- Error (3): Error conditions
- Fatal (4): Fatal errors

**Structured logging patterns:**
```go
// Good
log.Info().
    Str("user_id", userID).
    Int("attempts", retries).
    Msg("Authentication failed")

// Good - with context
log.Error().
    Err(err).
    Str("operation", "fetch_user").
    Msg("Database query failed")
```

**SpanLogger integration (from instrumentation):**
```go
// Create owned span - must call End()
ctx, logger := instrumentation.CreateLogSpan(ctx, "operation", "key", "value")
defer logger.End()

// Borrow current span - no End() method (compile-time safety)
view := instrumentation.CurrentSpanLogger(ctx)
view.Info("Helper logging")  // Cannot call view.End() - compile error
```

## Comments

**When to comment:**
- Package-level documentation explaining purpose and main concepts (always)
- Top-level types and exported functions must explain **why**, not just how
- Complex logic blocks that aren't self-documenting
- Non-obvious behavior: thread safety, nil parameter handling, special cases

**Format:**
- Package comments in separate file or at top of main file
- Exported function comments start with function name: `// FunctionName does X for Y reason`
- Include external references for unfamiliar patterns
- Examples in comments improve clarity (see godoc testable examples)

**GoDoc/JSDoc patterns:**
```go
// WithFileSink configures file output with automatic rotation.
// Use for traditional applications that need persistent logs.
// Overrideable via LOG_MODE, LOG_DIR, LOG_FILE_NAME environment variables.
//
// Example:
//
//	logr := logger.CreateLogger(
//	    logger.WithFileSink("/var/log", "app.log"),
//	    logger.WithRotation(100, 5, 28), // 100MB, 5 files, 28 days
//	)
func WithFileSink(dir, filename string) LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = FileMode
		c.Sink.Dir = dir
		c.Sink.FileName = filename
	}
}
```

**Testable examples:**
- Use `ExampleFunctionName()` in _test.go files (compiled and optionally executed)
- Show happy path only, no complex edge cases
- Output comments: `// Output: expected result`
- Excellent for executable documentation

## Function Design

**Size and Complexity:**
- Keep functions under 50 lines of code
- Max 2 nesting levels (deeply nested if/else: extract functions or use early returns)
- Cyclomatic complexity < 10 (enforced by linter)
- Cognitive complexity < 15 (enforced by linter)

**Single responsibility:**
- Each function should operate at a single conceptual level
- Don't mix low-level implementation details with high-level business logic
- If function reads like a comment block, extract it

**Story-like readability:**
- Top-level functions should read like a story - clear steps at a glance
- All steps should be understandable without diving into implementation
- Hide nitty-gritty details behind well-named functions and types

**Parameters:**
- Use functional options for complex configuration (e.g., `logger.CreateLogger(opts...)`)
- Avoid boolean parameters when possible; use semantic names or types
- Never pass `nil` - validate in constructor instead
- Context should be first parameter for IO operations

**Return values:**
- Use named return values when they improve clarity
- Error should always be last return value
- Return at most 2-3 values; use structs for multiple related values
- Never return nil values; return error instead

## Module Design

**Exports:**
- Only export what's part of the public API
- Unexported types and functions live alongside exported ones
- Use `_test` package for testing only the public API

**Type organization:**
- Group related types in type blocks:
```go
type (
	// spanLoggerBase contains shared fields
	spanLoggerBase struct { ... }
	
	// SpanLogger represents owned span
	SpanLogger struct { ... }
	
	// SpanLoggerView represents borrowed span
	SpanLoggerView struct { ... }
)
```

**Barrel files:**
- Use when package re-exports types from subpackages
- Keep minimal; prefer explicit imports in consumer code
- Not used heavily in this codebase

**Receiver naming:**
- Use short variable names (e.g., `s *SpanLogger`, `c *Config`)
- Consistent across all methods on a type
- Avoid `this`, `self` - use single letters

**Functional options pattern (heavily used):**
```go
// Define option type
type LoggerOption func(*Config)

// Define option creators
func WithFileSink(dir, filename string) LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = FileMode
		c.Sink.Dir = dir
		c.Sink.FileName = filename
	}
}

// Use with variadic parameter
func CreateLogger(opts ...LoggerOption) *Logger {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return NewZeroLoggerWithConfig(config)
}
```

## File Organization

**Declaration order (enforced by decorder linter):**
1. Constants
2. Variables
3. Types
4. Functions (init first if present, then public, then private)

**File structure:**
```
// Package documentation
package logger

// Imports (organized by gci)
import (...)

// Constants
const (...)

// Types
type SinkConfig struct {...}
type Config struct {...}

// Public functions
func NewLogger() {...}
func CreateLogger(...) {...}

// Private helper functions
func newInternalHelper() {...}
```

**Filename conventions:**
- One type per file (when type has methods): `type_name.go` (e.g., `spanlogger.go`)
- Related utilities grouped: `options.go`, `config.go`
- Tests: `_test.go` suffix with `_test` package name
- No underscores in package names

## Special Patterns

**Primitive obsession avoidance:**
- Create types for domain concepts instead of using primitives
- Example: `type TaskID string` with validation in constructor
- Type carries both data and behavior
- Makes code self-validating and type-safe

**Struct tags (enforced by tagliatelle):**
- Use snake_case for: `json`, `mapstructure`, `yaml`
- Example: `json:"max_size_mb"` not `json:"maxSizeMB"`

**Deferred functions:**
- If defer body has cyclomatic complexity > 1, extract to separate function
- Use defer for cleanup: defer resource.Close()
- Typical pattern: `defer logger.End()`

**Global state (gochecknoglobals enforces restrictions):**
- Generally forbidden
- Exceptions: `callerMarshalMutex` (thread-safe global state)
- Examples exclusion in `.golangci.yaml` for legitimate cases

---

*Convention analysis: 2026-07-07*
