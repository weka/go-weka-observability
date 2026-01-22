package logger_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/weka/go-weka-observability/logger"
)

type (
	// LoggerTestSuite is a testify suite for complex tests with file operations.
	LoggerTestSuite struct {
		suite.Suite
		origSlogHandler slog.Handler
		tempDir         string
	}

	// envOverrideTestCase defines a single environment override test case.
	envOverrideTestCase struct {
		check    func(*testing.T, logger.Config)
		name     string
		envKey   string
		envValue string
	}
)

// allLogEnvVars returns all LOG_* environment variables used in tests.
// Returned as a function to avoid package-level variable warnings.
func allLogEnvVars() []string {
	return []string{
		"LOG_MODE", "LOG_DIR", "LOG_FILE_NAME",
		"LOG_MAX_SIZE_MB", "LOG_MAX_FILES", "LOG_MAX_AGE_DAYS",
		"LOG_LEVEL", "LOG_FORMAT", "LOG_TIME_ONLY", "LOG_CALLER_DIR_LVL",
	}
}

// cleanupEnvVars removes specified environment variables
func cleanupEnvVars(t *testing.T, vars []string) {
	t.Helper()
	for _, v := range vars {
		if err := os.Unsetenv(v); err != nil {
			t.Logf("failed to unset %s: %v", v, err)
		}
	}
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

// Validation tests with slog capture

func (s *LoggerTestSuite) TestValidation_MissingLogFileName_EmitsSlogWarning() {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "", // Empty - should trigger warning
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	logger.NewZeroLoggerWithConfig(config)

	output := buf.String()
	s.Contains(output, "WARN")
	s.Contains(output, "FileMode requires FileName")
	s.Contains(output, "fallback=app.log")
}

func (s *LoggerTestSuite) TestValidation_MissingLogDir_EmitsSlogWarning() {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        "", // Empty - should trigger warning
			FileName:   "test.log",
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	logger.NewZeroLoggerWithConfig(config)

	output := buf.String()
	s.Contains(output, "WARN")
	s.Contains(output, "FileMode requires Dir")
	s.Contains(output, "fallback=")
	s.Contains(output, os.TempDir())
}

func (s *LoggerTestSuite) TestValidation_MissingLogFileName_UsesFallback() {
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "", // Empty
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	log := logger.NewZeroLoggerWithConfig(config)
	log.Info().Msg("test message")

	// Assert file created with fallback name
	fallbackPath := filepath.Join(s.tempDir, "app.log")
	s.FileExists(fallbackPath)
}

func (s *LoggerTestSuite) TestValidation_MissingLogDir_UsesFallback() {
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        "", // Empty
			FileName:   "test.log",
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	log := logger.NewZeroLoggerWithConfig(config)
	log.Info().Msg("test message")

	// Assert file created in os.TempDir() fallback
	fallbackPath := filepath.Join(os.TempDir(), "test.log")
	s.FileExists(fallbackPath)
}

func (s *LoggerTestSuite) TestFileMode_WithExplicitLogFileName_NoWarning() {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "myapp.log", // Explicit
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	logger.NewZeroLoggerWithConfig(config)

	output := buf.String()
	s.NotContains(output, "WARN")
	s.NotContains(output, "fallback")
}

// File mode integration tests

func (s *LoggerTestSuite) TestFileMode_WritesInfoAndErrorSeparately() {
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "test.log",
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	log := logger.NewZeroLoggerWithConfig(config)

	log.Info().Msg("info message")
	log.Warn().Msg("warn message")
	log.Error().Msg("error message")

	// Check info file
	infoPath := filepath.Join(s.tempDir, "test.log")
	s.FileExists(infoPath)
	infoContent, err := os.ReadFile(infoPath) //nolint:gosec // G304: test reads its own temp file
	s.Require().NoError(err)
	s.Contains(string(infoContent), "info message")
	s.NotContains(string(infoContent), "warn message")
	s.NotContains(string(infoContent), "error message")

	// Check error file
	errorPath := filepath.Join(s.tempDir, "test-error.log")
	s.FileExists(errorPath)
	errorContent, err := os.ReadFile(errorPath) //nolint:gosec // G304: test reads its own temp file
	s.Require().NoError(err)
	s.Contains(string(errorContent), "warn message")
	s.Contains(string(errorContent), "error message")
	s.NotContains(string(errorContent), "info message")
}

func (s *LoggerTestSuite) TestFileMode_CreatesCorrectFilenames() {
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "myapp.log",
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.DefaultFormatConfig(),
	}

	log := logger.NewZeroLoggerWithConfig(config)
	log.Info().Msg("test")
	log.Error().Msg("test")

	infoPath := filepath.Join(s.tempDir, "myapp.log")
	errorPath := filepath.Join(s.tempDir, "myapp-error.log")

	s.FileExists(infoPath)
	s.FileExists(errorPath)
}

func (s *LoggerTestSuite) TestEnvOverride_PartialOverride_RealFiles() {
	// Set only LOG_FILE_NAME env var
	s.Require().NoError(os.Setenv("LOG_FILE_NAME", "env-name.log"))

	custom := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "original.log", // Will be overridden
			MaxSizeMB:  200,
			MaxFiles:   3,
			MaxAgeDays: 7,
		},
		Format: logger.DefaultFormatConfig(),
	}

	config := logger.NewConfigFromEnv(custom)
	log := logger.NewZeroLoggerWithConfig(config)

	log.Info().Msg("test message")

	// Assert file created with ENV overridden name
	envPath := filepath.Join(s.tempDir, "env-name.log")
	s.FileExists(envPath)

	// Assert file NOT created with original name
	originalPath := filepath.Join(s.tempDir, "original.log")
	_, err := os.Stat(originalPath)
	s.True(os.IsNotExist(err))
}

func (s *LoggerTestSuite) TestMultiLevelWriter_SeparatesLevels() {
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "levels.log",
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: logger.FormatConfig{
			Level:        zerolog.TraceLevel, // Set to trace to capture all levels
			Format:       logger.LogFormatJSON,
			TimeOnly:     false,
			CallerDirLvl: -1,
		},
	}

	log := logger.NewZeroLoggerWithConfig(config)

	// Write all levels
	log.Trace().Msg("trace")
	log.Debug().Msg("debug")
	log.Info().Msg("info")
	log.Warn().Msg("warn")
	log.Error().Msg("error")

	// Info file should have trace, debug, info
	infoPath := filepath.Join(s.tempDir, "levels.log")
	infoContent, err := os.ReadFile(infoPath) //nolint:gosec // G304: test reads its own temp file
	s.Require().NoError(err)
	infoStr := string(infoContent)
	s.Contains(infoStr, "trace")
	s.Contains(infoStr, "debug")
	s.Contains(infoStr, "info")
	s.NotContains(infoStr, "warn")
	s.NotContains(infoStr, "error")

	// Error file should have warn, error
	errorPath := filepath.Join(s.tempDir, "levels-error.log")
	errorContent, readErr := os.ReadFile(errorPath) //nolint:gosec // G304: test reads its own temp file
	s.Require().NoError(readErr)
	errorStr := string(errorContent)
	s.Contains(errorStr, "warn")
	s.Contains(errorStr, "error")
	s.NotContains(errorStr, "trace")
	s.NotContains(errorStr, "debug")
	s.NotContains(errorStr, "info")
}

func (s *LoggerTestSuite) TestNewZeroLoggerWithConfig_UsesProvidedConfig() {
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode:       logger.FileMode,
			Dir:        s.tempDir,
			FileName:   "custom-config.log",
			MaxSizeMB:  50,
			MaxFiles:   3,
			MaxAgeDays: 7,
		},
		Format: logger.FormatConfig{
			Level:        zerolog.InfoLevel,
			Format:       logger.LogFormatJSON,
			TimeOnly:     false,
			CallerDirLvl: -1,
		},
	}

	log := logger.NewZeroLoggerWithConfig(config)
	log.Info().Msg("custom config test")

	customPath := filepath.Join(s.tempDir, "custom-config.log")
	s.FileExists(customPath)
}

// Tests for new Config type

func TestDefaultConfig(t *testing.T) {
	config := logger.DefaultConfig()

	// Test sink defaults
	assert.Equal(t, logger.ConsoleMode, config.Sink.Mode)
	assert.Equal(t, "/var/log", config.Sink.Dir)
	assert.Empty(t, config.Sink.FileName)
	assert.Equal(t, 100, config.Sink.MaxSizeMB)
	assert.Equal(t, 5, config.Sink.MaxFiles)
	assert.Equal(t, 28, config.Sink.MaxAgeDays)

	// Test format defaults
	assert.Equal(t, zerolog.InfoLevel, config.Format.Level)
	assert.Equal(t, logger.LogFormatJSON, config.Format.Format)
	assert.False(t, config.Format.TimeOnly)
	assert.Equal(t, -1, config.Format.CallerDirLvl)
}

// runEnvOverrideTests executes environment override test cases with proper cleanup.
func runEnvOverrideTests(t *testing.T, tests []envOverrideTestCase) {
	t.Helper()
	cleanupEnvVars(t, allLogEnvVars())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupEnvVars(t, allLogEnvVars())

			require.NoError(t, os.Setenv(tt.envKey, tt.envValue))
			defer func() {
				if err := os.Unsetenv(tt.envKey); err != nil {
					t.Log(err)
				}
			}()

			config := logger.NewConfigFromEnv(logger.DefaultConfig())
			tt.check(t, config)
		})
	}
}

func TestNewConfigFromEnv_SinkOverrides(t *testing.T) {
	tests := []envOverrideTestCase{
		{
			name: "LOG_MODE overrides sink mode", envKey: "LOG_MODE", envValue: "file",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, logger.FileMode, c.Sink.Mode) },
		},
		{
			name: "LOG_DIR overrides sink directory", envKey: "LOG_DIR", envValue: "/custom/dir",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, "/custom/dir", c.Sink.Dir) },
		},
		{
			name: "LOG_FILE_NAME overrides sink filename", envKey: "LOG_FILE_NAME", envValue: "custom.log",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, "custom.log", c.Sink.FileName) },
		},
		{
			name: "LOG_MAX_SIZE_MB overrides sink max size", envKey: "LOG_MAX_SIZE_MB", envValue: "50",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, 50, c.Sink.MaxSizeMB) },
		},
		{
			name: "LOG_MAX_FILES overrides sink max files", envKey: "LOG_MAX_FILES", envValue: "10",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, 10, c.Sink.MaxFiles) },
		},
		{
			name: "LOG_MAX_AGE_DAYS overrides sink max age", envKey: "LOG_MAX_AGE_DAYS", envValue: "7",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, 7, c.Sink.MaxAgeDays) },
		},
	}
	runEnvOverrideTests(t, tests)
}

func TestNewConfigFromEnv_FormatOverrides(t *testing.T) {
	tests := []envOverrideTestCase{
		{
			name: "LOG_LEVEL numeric overrides format level", envKey: "LOG_LEVEL", envValue: "0",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, zerolog.DebugLevel, c.Format.Level) },
		},
		{
			name: "LOG_LEVEL string INFO overrides format level", envKey: "LOG_LEVEL", envValue: "INFO",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, zerolog.InfoLevel, c.Format.Level) },
		},
		{
			name: "LOG_LEVEL string debug (lowercase) overrides format level", envKey: "LOG_LEVEL", envValue: "debug",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, zerolog.DebugLevel, c.Format.Level) },
		},
		{
			name: "LOG_LEVEL string warn overrides format level", envKey: "LOG_LEVEL", envValue: "warn",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, zerolog.WarnLevel, c.Format.Level) },
		},
		{
			name: "LOG_FORMAT overrides format", envKey: "LOG_FORMAT", envValue: "raw",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, logger.LogFormatRaw, c.Format.Format) },
		},
		{
			name: "LOG_TIME_ONLY overrides time format", envKey: "LOG_TIME_ONLY", envValue: "true",
			check: func(t *testing.T, c logger.Config) { assert.True(t, c.Format.TimeOnly) },
		},
		{
			name: "LOG_CALLER_DIR_LVL overrides caller dir level", envKey: "LOG_CALLER_DIR_LVL", envValue: "2",
			check: func(t *testing.T, c logger.Config) { assert.Equal(t, 2, c.Format.CallerDirLvl) },
		},
	}
	runEnvOverrideTests(t, tests)
}

func TestCreateLogger_WithConsoleSink(t *testing.T) {
	// Apply option to config to verify it works
	config := logger.DefaultConfig()
	logger.WithConsoleSink()(&config)

	assert.Equal(t, logger.ConsoleMode, config.Sink.Mode)
}

func TestCreateLogger_WithFileSink(t *testing.T) {
	config := logger.DefaultConfig()
	logger.WithFileSink("/tmp", "test.log")(&config)

	assert.Equal(t, logger.FileMode, config.Sink.Mode)
	assert.Equal(t, "/tmp", config.Sink.Dir)
	assert.Equal(t, "test.log", config.Sink.FileName)
}

func TestCreateLogger_WithRotation(t *testing.T) {
	config := logger.DefaultConfig()
	logger.WithRotation(50, 10, 7)(&config)

	assert.Equal(t, 50, config.Sink.MaxSizeMB)
	assert.Equal(t, 10, config.Sink.MaxFiles)
	assert.Equal(t, 7, config.Sink.MaxAgeDays)
}

func TestCreateLogger_WithJSONFormat(t *testing.T) {
	config := logger.DefaultConfig()
	logger.WithJSONFormat()(&config)

	assert.Equal(t, logger.LogFormatJSON, config.Format.Format)
}

func TestCreateLogger_WithDebugLevel(t *testing.T) {
	config := logger.DefaultConfig()
	logger.WithDebugLevel()(&config)

	assert.Equal(t, zerolog.DebugLevel, config.Format.Level)
}

func TestCreateLogger_WithMultipleOptions(t *testing.T) {
	config := logger.DefaultConfig()
	logger.WithFileSink("/var/log", "app.log")(&config)
	logger.WithInfoLevel()(&config)
	logger.WithJSONFormat()(&config)
	logger.WithRotation(100, 5, 28)(&config)

	assert.Equal(t, logger.FileMode, config.Sink.Mode)
	assert.Equal(t, "/var/log", config.Sink.Dir)
	assert.Equal(t, "app.log", config.Sink.FileName)
	assert.Equal(t, zerolog.InfoLevel, config.Format.Level)
	assert.Equal(t, logger.LogFormatJSON, config.Format.Format)
	assert.Equal(t, 100, config.Sink.MaxSizeMB)
	assert.Equal(t, 5, config.Sink.MaxFiles)
	assert.Equal(t, 28, config.Sink.MaxAgeDays)
}

// Test parsing functions

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
			name:      "valid file",
			input:     "file",
			want:      logger.FileMode,
			wantError: false,
		},
		{
			name:      "invalid mode",
			input:     "invalid",
			want:      logger.ConsoleMode, // returns default
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			want:      logger.ConsoleMode,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := logger.ParseOutputMode(tt.input)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid output mode")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseLogFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      logger.LogFormat
		wantError bool
	}{
		{
			name:      "valid raw",
			input:     "raw",
			want:      logger.LogFormatRaw,
			wantError: false,
		},
		{
			name:      "valid json",
			input:     "json",
			want:      logger.LogFormatJSON,
			wantError: false,
		},
		{
			name:      "valid plain",
			input:     "plain",
			want:      logger.LogFormatPlain,
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     "xml",
			want:      logger.LogFormatJSON, // returns default
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			want:      logger.LogFormatJSON,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := logger.ParseLogFormat(tt.input)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid log format")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetStderrWriter_InvalidLogFormat(t *testing.T) {
	// Test that invalid LOG_FORMAT values are handled gracefully and fallback to default
	if err := os.Setenv("LOG_FORMAT", "invalid_format"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Unsetenv("LOG_FORMAT"); err != nil {
			t.Logf("failed to unset LOG_FORMAT: %v", err)
		}
	}()

	// Should not panic, should return a valid writer
	writer := logger.GetStderrWriter()
	assert.NotNil(t, writer)

	// Verify it actually uses default format (JSON -> os.Stderr)
	// The default JSON format returns os.Stderr directly
	assert.Equal(t, os.Stderr, writer, "should fallback to JSON format which returns os.Stderr")
}

func TestGetStderrWriter_JSONFormat(t *testing.T) {
	if err := os.Setenv("LOG_FORMAT", "json"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Unsetenv("LOG_FORMAT"); err != nil {
			t.Logf("failed to unset LOG_FORMAT: %v", err)
		}
	}()

	writer := logger.GetStderrWriter()
	assert.NotNil(t, writer)
	assert.Equal(t, os.Stderr, writer, "JSON format should return os.Stderr")
}

func TestGetStderrWriter_RawFormat(t *testing.T) {
	if err := os.Setenv("LOG_FORMAT", "raw"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Unsetenv("LOG_FORMAT"); err != nil {
			t.Logf("failed to unset LOG_FORMAT: %v", err)
		}
	}()

	writer := logger.GetStderrWriter()
	assert.NotNil(t, writer)
	assert.NotEqual(t, os.Stderr, writer, "Raw format should return ConsoleWriter")
}

func TestGetStderrWriter_PlainFormat(t *testing.T) {
	if err := os.Setenv("LOG_FORMAT", "plain"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Unsetenv("LOG_FORMAT"); err != nil {
			t.Logf("failed to unset LOG_FORMAT: %v", err)
		}
	}()

	writer := logger.GetStderrWriter()
	assert.NotNil(t, writer)
	assert.NotEqual(t, os.Stderr, writer, "Plain format should return ConsoleWriter")
}

// Test caller functionality

func TestCallerDirDisplayLevel_WithConfig(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		callerDirLvl int
		expectCaller bool
	}{
		{
			name:         "caller disabled with -1",
			callerDirLvl: -1,
			expectCaller: false,
		},
		{
			name:         "caller enabled with 0",
			callerDirLvl: 0,
			expectCaller: true,
		},
		{
			name:         "caller enabled with 2",
			callerDirLvl: 2,
			expectCaller: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logFile := filepath.Join(tempDir, tt.name+".log")

			config := logger.Config{
				Sink: logger.SinkConfig{
					Mode:       logger.FileMode,
					Dir:        tempDir,
					FileName:   tt.name + ".log",
					MaxSizeMB:  100,
					MaxFiles:   5,
					MaxAgeDays: 28,
				},
				Format: logger.FormatConfig{
					Level:        zerolog.InfoLevel,
					Format:       logger.LogFormatJSON,
					TimeOnly:     false,
					CallerDirLvl: tt.callerDirLvl,
				},
			}

			log := logger.NewZeroLoggerWithConfig(config)
			log.Info().Msg("test message")

			content, err := os.ReadFile(logFile) //nolint:gosec // G304: test reads its own temp file
			require.NoError(t, err)

			if tt.expectCaller {
				assert.Contains(t, string(content), `"caller"`)
				assert.Contains(t, string(content), "logger_test.go:")
			} else {
				assert.NotContains(t, string(content), `"caller"`)
			}
		})
	}
}

// testDeprecatedCallerSetup creates a config for testing deprecated caller functionality.
func testDeprecatedCallerSetup(tempDir string) logger.Config {
	return logger.Config{
		Sink: logger.SinkConfig{
			Mode: logger.FileMode, Dir: tempDir, FileName: "deprecated-caller.log",
			MaxSizeMB: 100, MaxFiles: 5, MaxAgeDays: 28,
		},
		Format: logger.FormatConfig{
			Level: zerolog.InfoLevel, Format: logger.LogFormatJSON, TimeOnly: false,
			CallerDirLvl: 0, // Enable caller in config
		},
	}
}

// readDeprecatedCallerLog reads the log file created by deprecated caller tests.
//
//nolint:gosec // G304: Test helper reads known temp file path controlled by test
func readDeprecatedCallerLog(t *testing.T, tempDir string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(tempDir, "deprecated-caller.log"))
	require.NoError(t, err)

	return content
}

func TestSetCallerDirDisplayLevel_EnvNotSet(t *testing.T) {
	if err := os.Unsetenv("LOG_CALLER_DIR_LVL"); err != nil {
		t.Logf("failed to unset LOG_CALLER_DIR_LVL: %v", err)
	}

	tempDir := t.TempDir()
	logger.SetCallerDirDisplayLevel()

	log := logger.NewZeroLoggerWithConfig(testDeprecatedCallerSetup(tempDir))
	log.Info().Msg("test deprecated caller")

	content := readDeprecatedCallerLog(t, tempDir)
	assert.Contains(t, string(content), "caller")
	assert.Contains(t, string(content), "logger_test.go:")
}

func TestSetCallerDirDisplayLevel_EnvSetToZero(t *testing.T) {
	if err := os.Unsetenv("LOG_CALLER_DIR_LVL"); err != nil {
		t.Logf("failed to unset LOG_CALLER_DIR_LVL: %v", err)
	}
	require.NoError(t, os.Setenv("LOG_CALLER_DIR_LVL", "0"))
	defer func() {
		if err := os.Unsetenv("LOG_CALLER_DIR_LVL"); err != nil {
			t.Logf("failed to unset LOG_CALLER_DIR_LVL: %v", err)
		}
	}()

	tempDir := t.TempDir()
	logger.SetCallerDirDisplayLevel()

	log := logger.NewZeroLoggerWithConfig(testDeprecatedCallerSetup(tempDir))
	log.Info().Msg("test deprecated caller")

	content := readDeprecatedCallerLog(t, tempDir)
	assert.Contains(t, string(content), "caller")
	assert.Contains(t, string(content), "logger_test.go:")
}

func TestSetCallerDirDisplayLevel_EnvSetToTwo(t *testing.T) {
	if err := os.Unsetenv("LOG_CALLER_DIR_LVL"); err != nil {
		t.Logf("failed to unset LOG_CALLER_DIR_LVL: %v", err)
	}
	require.NoError(t, os.Setenv("LOG_CALLER_DIR_LVL", "2"))
	defer func() {
		if err := os.Unsetenv("LOG_CALLER_DIR_LVL"); err != nil {
			t.Logf("failed to unset LOG_CALLER_DIR_LVL: %v", err)
		}
	}()

	tempDir := t.TempDir()
	logger.SetCallerDirDisplayLevel()

	log := logger.NewZeroLoggerWithConfig(testDeprecatedCallerSetup(tempDir))
	log.Info().Msg("test deprecated caller")

	content := readDeprecatedCallerLog(t, tempDir)
	assert.Contains(t, string(content), "caller")
	assert.Contains(t, string(content), "logger_test.go:")
}

func TestNewZeroLoggerWithConfig_ConsoleModeRoutesToStdout(t *testing.T) {
	config := logger.Config{
		Sink:   logger.SinkConfig{Mode: logger.ConsoleMode},
		Format: logger.FormatConfig{Format: logger.LogFormatJSON, Level: zerolog.TraceLevel},
	}

	// Capture stdout using os.Pipe
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	log := logger.NewZeroLoggerWithConfig(config)
	testMsg := "newzerologger_info_test_11111"
	log.Info().Msg(testMsg)

	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Contains(t, string(output), testMsg, "NewZeroLoggerWithConfig should route info to stdout")
}

func TestNewZeroLoggerWithConfig_ConsoleModeRoutesToStderr(t *testing.T) {
	config := logger.Config{
		Sink:   logger.SinkConfig{Mode: logger.ConsoleMode},
		Format: logger.FormatConfig{Format: logger.LogFormatJSON, Level: zerolog.TraceLevel},
	}

	// Capture stderr using os.Pipe
	origStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	log := logger.NewZeroLoggerWithConfig(config)
	testMsg := "newzerologger_error_test_22222"
	log.Error().Msg(testMsg)

	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Contains(t, string(output), testMsg, "NewZeroLoggerWithConfig should route error to stderr")
}

func TestNewZeroLoggerWithConfig_FileModeWritesToFiles(t *testing.T) {
	tempDir := t.TempDir()
	config := logger.Config{
		Sink: logger.SinkConfig{
			Mode: logger.FileMode, Dir: tempDir, FileName: "test.log",
			MaxSizeMB: 10, MaxFiles: 3, MaxAgeDays: 7,
		},
		Format: logger.FormatConfig{Format: logger.LogFormatJSON, Level: zerolog.TraceLevel},
	}

	log := logger.NewZeroLoggerWithConfig(config)
	infoMsg, errorMsg := "file_info_test_33333", "file_error_test_44444"
	log.Info().Msg(infoMsg)
	log.Error().Msg(errorMsg)

	infoFile := filepath.Join(tempDir, "test.log")
	errorFile := filepath.Join(tempDir, "test-error.log")

	infoContent, err := os.ReadFile(infoFile) //nolint:gosec // G304: test reads its own temp file
	require.NoError(t, err)
	assert.Contains(t, string(infoContent), infoMsg, "Info log should be in info file")
	assert.NotContains(t, string(infoContent), errorMsg, "Error log should NOT be in info file")

	errorContent, err := os.ReadFile(errorFile) //nolint:gosec // G304: test reads its own temp file
	require.NoError(t, err)
	assert.Contains(t, string(errorContent), errorMsg, "Error log should be in error file")
	assert.NotContains(t, string(errorContent), infoMsg, "Info log should NOT be in error file")
}

// Tests for GetLogLevel (deprecated function)

func TestGetLogLevel_StringLevelNames(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     zerolog.Level
	}{
		{name: "uppercase INFO", envValue: "INFO", want: zerolog.InfoLevel},
		{name: "lowercase info", envValue: "info", want: zerolog.InfoLevel},
		{name: "uppercase DEBUG", envValue: "DEBUG", want: zerolog.DebugLevel},
		{name: "lowercase debug", envValue: "debug", want: zerolog.DebugLevel},
		{name: "uppercase WARN", envValue: "WARN", want: zerolog.WarnLevel},
		{name: "lowercase warn", envValue: "warn", want: zerolog.WarnLevel},
		{name: "uppercase ERROR", envValue: "ERROR", want: zerolog.ErrorLevel},
		{name: "lowercase error", envValue: "error", want: zerolog.ErrorLevel},
		{name: "uppercase TRACE", envValue: "TRACE", want: zerolog.TraceLevel},
		{name: "lowercase trace", envValue: "trace", want: zerolog.TraceLevel},
		{name: "numeric 0 (debug)", envValue: "0", want: zerolog.DebugLevel},
		{name: "numeric 1 (info)", envValue: "1", want: zerolog.InfoLevel},
		{name: "numeric -1 (trace)", envValue: "-1", want: zerolog.TraceLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupEnvVars(t, allLogEnvVars())

			require.NoError(t, os.Setenv("LOG_LEVEL", tt.envValue))
			defer func() {
				if err := os.Unsetenv("LOG_LEVEL"); err != nil {
					t.Log(err)
				}
			}()

			got := logger.GetLogLevel()
			assert.Equal(t, tt.want, got, "GetLogLevel() should parse %q as %v", tt.envValue, tt.want)
		})
	}
}
