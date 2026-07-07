# Testing Patterns

**Analysis Date:** 2026-07-07

## Test Framework

**Runner:**
- Go's built-in testing package (`testing`)
- Race detector enabled: `go test -race ./...`
- Config: none (uses defaults)

**Assertion Library:**
- testify/assert - soft assertions that don't stop test execution
- testify/require - hard assertions that fail immediately
- testify/suite - for complex setup scenarios

**Run Commands:**
```bash
task test                # Run all tests with race detector
task test-coverage       # Run tests with coverage report (coverage.html)
task test-verbose        # Run tests with verbose output (-v)
go test -race ./...      # Direct invocation
```

**Test structure verified by:**
- testifylint linter (checks for common testify mistakes)
- tparallel linter (detects inappropriate t.Parallel() usage)

## Test File Organization

**Location:**
- Tests live alongside implementation (co-located in same package)
- Test files: `{filename}_test.go` (e.g., `logger_test.go`)
- No separate `test/` directory

**Package naming:**
- Implementation: `package logger`
- Tests: `package logger_test` (tests only public API)
- Ensures tests verify exported API only

**File structure example:**
```
logger/
├── logger.go           # Implementation
├── logger_test.go      # Tests
├── config.go
├── config_test.go      # If tests need to be separate
└── options.go
```

## Test Structure

**Suite-based tests (for complex infrastructure):**
```go
type LoggerTestSuite struct {
	suite.Suite
	origSlogHandler slog.Handler
	tempDir         string
}

func (s *LoggerTestSuite) SetupTest() {
	s.tempDir = s.T().TempDir()
	s.origSlogHandler = slog.Default().Handler()
}

func (s *LoggerTestSuite) TearDownTest() {
	slog.SetDefault(slog.New(s.origSlogHandler))
	cleanupEnvVars(s.T(), allLogEnvVars())
}

func TestLoggerSuite(t *testing.T) {
	suite.Run(t, new(LoggerTestSuite))
}

// Test methods on suite
func (s *LoggerTestSuite) TestValidation_MissingLogDir_EmitsSlogWarning() {
	// test implementation
}
```

**Table-driven tests (for simple cases):**
```go
func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      logger.OutputMode
		wantError bool
	}{
		{
			name:      "valid console",
			input:     "console",
			want:      logger.ConsoleMode,
			wantError: false,
		},
		{
			name:      "invalid mode",
			input:     "invalid",
			want:      logger.ConsoleMode,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := logger.ParseOutputMode(tt.input)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
```

**CRITICAL: Always use named fields in table-driven tests:**
```go
// ❌ BAD - breaks when linter reorders struct fields
tests := []struct {
	name   string
	input  int
	want   string
}{
	{"test1", 42, "result"},  // Positional - breaks on field reorder
}

// ✅ GOOD - works regardless of field order
tests := []struct {
	name   string
	input  int
	want   string
}{
	{name: "test1", input: 42, want: "result"},  // Always works
}
```

**When to use suites vs table-driven:**
- **Use suites** for: Mock servers, databases, external services, OTel setup, temp files, shared expensive setup
- **Use table-driven** for: Simple unit tests with one scenario per row, no complex setup
- **Avoid mixing:** Inside `t.Run()`, keep complexity = 1 (no if/else, switch, etc.)

## Mocking Strategy

**Real implementations over mocks (integration-style testing):**
- Use http.test servers instead of mocking HTTP
- Use temp files/directories (via `t.TempDir()`) instead of mocking filesystem
- Use in-memory test doubles when real implementations are unavailable

**Example patterns from codebase:**
```go
// Good: Real file I/O in tests
func TestFileMode_WritesInfoAndErrorSeparately() {
	tempDir := s.T().TempDir()  // Real temp directory
	log := logger.NewZeroLoggerWithConfig(config)
	log.Info().Msg("info message")
	log.Error().Msg("error message")

	// Verify with real file reads
	infoContent, err := os.ReadFile(filepath.Join(tempDir, "test.log"))
	require.NoError(t, err)
	assert.Contains(t, string(infoContent), "info message")
}

// Good: Real OTel instrumentation for tracing tests
func TestGetTracerProviderDetection(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	
	ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "test-op")
	spanLogger.End()
	
	spans := recorder.Ended()
	require.Len(t, spans, 1)
}
```

**When to use test doubles:**
- `tracetest.SpanRecorder` for OpenTelemetry span recording
- `bytes.Buffer` for capturing log output

## Test Helpers

**Common helper patterns (from codebase):**

```go
// Test setup helpers
func testLogConfig() logger.Config {
	return logger.Config{
		Sink:   logger.SinkConfig{Mode: logger.ConsoleMode},
		Format: logger.FormatConfig{Format: logger.LogFormatJSON},
	}
}

// Environment cleanup helpers
func cleanupEnvVars(t *testing.T, vars []string) {
	t.Helper()
	for _, v := range vars {
		if err := os.Unsetenv(v); err != nil {
			t.Logf("failed to unset %s: %v", v, err)
		}
	}
}

// Capture output helpers
func captureStdoutWithMultiLevelWriter(t *testing.T, testMsg string, level zerolog.Level) []byte {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	writer := logger.GetMultiLevelWriterWithConfig(config)
	log := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	log.Msg(testMsg)

	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)
	return output
}

// Provider shutdown helpers
func shutdownProvider(ctx context.Context, t *testing.T, tp *trace.TracerProvider) {
	t.Helper()
	if err := tp.Shutdown(ctx); err != nil {
		t.Logf("failed to shutdown provider: %v", err)
	}
}
```

**Helper guidelines:**
- Always call `t.Helper()` at start of test helper
- Use `require.` for assertions that should fail immediately in helpers
- Document purpose with comments

## Test Data & Fixtures

**Pattern from codebase:**
```go
// Inline test case structs with all fields named
type envOverrideTestCase struct {
	check    func(*testing.T, logger.Config)
	name     string
	envKey   string
	envValue string
}

// Inline test data
tests := []envOverrideTestCase{
	{
		name: "LOG_MODE overrides sink mode",
		envKey: "LOG_MODE",
		envValue: "file",
		check: func(t *testing.T, c logger.Config) {
			assert.Equal(t, logger.FileMode, c.Sink.Mode)
		},
	},
}
```

**Fixtures location:**
- Inline in test files when small (under 50 lines)
- Helper functions when reused across tests
- No separate fixture files in this codebase

## Coverage

**Requirements:** None explicitly enforced, but comprehensive

**View coverage:**
```bash
task test-coverage    # Generates coverage.html
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

**Coverage practices:**
- Focus on testing public API only (package_test naming ensures this)
- Each exported function should have representative tests
- Edge cases and error paths should be covered

## Test Types

**Unit Tests (majority):**
- Scope: Single public API of one type/function
- Approach: Initialize via constructors, call public methods
- Real dependencies (no mocks)
- Example: `TestParseOutputMode` - tests single parsing function

**Integration Tests (in suites):**
- Scope: Multiple components working together
- Example: Logger creation → file rotation → multi-level writing
- Uses real filesystem, real I/O
- May have overlapping coverage with unit tests

**OTel Tests (tracetest package):**
- Uses `tracetest.SpanRecorder` to record spans
- Tests span creation, context propagation, span attributes
- Real OpenTelemetry SDK (not mocked)

## Common Patterns

**Async/context testing (no time.Sleep):**
```go
// Instead of:
// time.Sleep(100*time.Millisecond)

// Use wait groups or channels for signal-based synchronization
// (not shown in this codebase, but linter warns about Sleep in tests)
```

**Error testing (requires checking error message content):**
```go
func TestNewConfigFromEnv_InvalidLogFormat(t *testing.T) {
	require.NoError(t, os.Setenv("LOG_FORMAT", "invalid"))
	defer os.Unsetenv("LOG_FORMAT")

	got, err := logger.ParseLogFormat("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log format")
	assert.Equal(t, logger.LogFormatJSON, got)  // default returned
}
```

**Environment variable testing (cleanup after each test):**
```go
func (s *LoggerTestSuite) TestEnvOverride_PartialOverride_RealFiles() {
	s.Require().NoError(os.Setenv("LOG_FILE_NAME", "env-name.log"))
	// Test runs

	// Cleanup happens in TearDownTest() for suite, or:
}

// For standalone functions:
func TestSomeEnvBased(t *testing.T) {
	require.NoError(t, os.Setenv("LOG_MODE", "file"))
	defer func() {
		if err := os.Unsetenv("LOG_MODE"); err != nil {
			t.Logf("failed to unset LOG_MODE: %v", err)
		}
	}()
}
```

**File I/O testing (with temp directories):**
```go
func TestFileMode_WritesToFiles(t *testing.T) {
	tempDir := t.TempDir()  // Automatic cleanup

	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode: logger.FileMode,
			Dir: tempDir,
			FileName: "test.log",
		},
	}

	log := logger.NewZeroLoggerWithConfig(config)
	log.Info().Msg("test message")

	// Assert file exists and has content
	content, err := os.ReadFile(filepath.Join(tempDir, "test.log"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message")
}
```

**OTel span testing (with SpanRecorder):**
```go
func TestCreateLogSpan(t *testing.T) {
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)

	ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "test-op", "key", "value")
	spanLogger.End()

	// Verify span was recorded
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-op", spans[0].Name())

	// Cleanup
	if err := tp.Shutdown(ctx); err != nil {
		t.Logf("shutdown error: %v", err)
	}
}
```

## Test Execution

**Pre-commit hooks (via `.pre-commit-config.yaml`):**
- Runs `task fmt` (formatting)
- Runs `task lintfix-no-output` (linting with fixes)
- Prevents commits with linting errors

**Linting in tests:**
- testifylint: Checks for common testify mistakes (e.g., wrong assertion ordering)
- tparallel: Detects `t.Parallel()` usage that would cause issues

## Test Quality Rules

**Principles from CLAUDE.md:**
1. Test only the public API of the package
2. Use testify for testing
3. Avoid mocks - use real implementations
4. Avoid time.Sleep - use channels or wait groups
5. Prefer simple, straightforward solutions
6. Each test should test exactly one scenario (cyclomatic complexity = 1)
7. Separate success and error cases into different test functions

**Separate test functions by outcome:**
```go
// ✓ Good - separate functions for success and error paths
func TestParseLogFormat_ValidFormats(t *testing.T) {
	// Test happy path cases
}

func TestParseLogFormat_InvalidFormats(t *testing.T) {
	// Test error cases
}

// ✗ Avoid - mixing success and error in one test with if/else
func TestParseLogFormat(t *testing.T) {
	if valid {
		// success path
	} else {
		// error path
	}
}
```

---

*Testing analysis: 2026-07-07*
