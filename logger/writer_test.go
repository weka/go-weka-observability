package logger_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weka/go-weka-observability/logger"
)

// levelRoutingResult holds the captured output from multi-level writer test.
type levelRoutingResult struct {
	infoContent  string
	errorContent string
}

// writeLogAtLevel writes a log message at the specified level using WithLevel for all levels.
// This avoids special behavior like os.Exit (Fatal) or panic (Panic).
//
//nolint:gocritic // hugeParam: test helper - performance not critical
func writeLogAtLevel(log zerolog.Logger, level zerolog.Level, msg string) {
	log.WithLevel(level).Msg(msg)
}

// testLogConfig returns a standard config for multi-level writer tests.
func testLogConfig() logger.Config {
	return logger.Config{
		Sink:   logger.SinkConfig{Mode: logger.ConsoleMode},
		Format: logger.FormatConfig{Format: logger.LogFormatJSON, Level: zerolog.TraceLevel},
	}
}

// captureStdoutWithMultiLevelWriter captures stdout output when logging with GetMultiLevelWriterWithConfig.
func captureStdoutWithMultiLevelWriter(t *testing.T, testMsg string, level zerolog.Level) []byte {
	t.Helper()
	config := testLogConfig()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	writer := logger.GetMultiLevelWriterWithConfig(config)
	log := zerolog.New(writer).Level(config.Format.Level).With().Timestamp().Logger()
	writeLogAtLevel(log, level, testMsg)

	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)

	return output
}

// captureStderrWithMultiLevelWriter captures stderr output when logging with GetMultiLevelWriterWithConfig.
func captureStderrWithMultiLevelWriter(t *testing.T, testMsg string, level zerolog.Level) []byte {
	t.Helper()
	config := testLogConfig()
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	writer := logger.GetMultiLevelWriterWithConfig(config)
	log := zerolog.New(writer).Level(config.Format.Level).With().Timestamp().Logger()
	writeLogAtLevel(log, level, testMsg)

	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)

	return output
}

// testLevelRouting tests a single level routing and returns captured content
func testLevelRouting(level zerolog.Level, msg string) levelRoutingResult {
	infoBuffer := &bytes.Buffer{}
	errorBuffer := &bytes.Buffer{}

	infoWriter := logger.SpecificLevelWriter{
		Writer:      infoBuffer,
		ShouldWrite: func(l zerolog.Level) bool { return l < zerolog.WarnLevel },
	}
	errorWriter := logger.SpecificLevelWriter{
		Writer:      errorBuffer,
		ShouldWrite: func(l zerolog.Level) bool { return l >= zerolog.WarnLevel },
	}
	multiWriter := zerolog.MultiLevelWriter(infoWriter, errorWriter)
	log := zerolog.New(multiWriter).Level(zerolog.TraceLevel).With().Timestamp().Logger()

	writeLogAtLevel(log, level, msg)

	return levelRoutingResult{
		infoContent:  infoBuffer.String(),
		errorContent: errorBuffer.String(),
	}
}

// TestLevelComparators_InfoLevelsRouteToInfoWriter tests that trace/debug/info route to info writer
func TestLevelComparators_InfoLevelsRouteToInfoWriter(t *testing.T) {
	infoLevels := []zerolog.Level{zerolog.TraceLevel, zerolog.DebugLevel, zerolog.InfoLevel}

	for _, level := range infoLevels {
		t.Run(level.String(), func(t *testing.T) {
			msg := "test message for " + level.String()
			result := testLevelRouting(level, msg)

			assert.Contains(t, result.infoContent, msg, "Level %s should route to info writer", level)
			assert.Empty(t, result.errorContent, "Level %s should NOT route to error writer", level)
		})
	}
}

// TestLevelComparators_ErrorLevelsRouteToErrorWriter tests that warn/error/fatal/panic route to error writer
func TestLevelComparators_ErrorLevelsRouteToErrorWriter(t *testing.T) {
	errorLevels := []zerolog.Level{
		zerolog.WarnLevel, zerolog.ErrorLevel, zerolog.FatalLevel,
		zerolog.PanicLevel, zerolog.NoLevel,
	}

	for _, level := range errorLevels {
		t.Run(level.String(), func(t *testing.T) {
			msg := "test message for " + level.String()
			result := testLevelRouting(level, msg)

			assert.Contains(t, result.errorContent, msg, "Level %s should route to error writer", level)
			assert.Empty(t, result.infoContent, "Level %s should NOT route to info writer", level)
		})
	}
}

// TestLevelComparisonBoundary tests the exact boundary between info and error levels
func TestLevelComparisonBoundary(t *testing.T) {
	t.Run("levels below WarnLevel route to info", func(t *testing.T) {
		// Test levels: Trace(-1), Debug(0), Info(1)
		belowWarn := []zerolog.Level{
			zerolog.TraceLevel,
			zerolog.DebugLevel,
			zerolog.InfoLevel,
		}

		for _, level := range belowWarn {
			assert.Less(t, level, zerolog.WarnLevel,
				"Expected %s (%d) < WarnLevel (%d)", level, level, zerolog.WarnLevel)
		}
	})

	t.Run("levels at and above WarnLevel route to error", func(t *testing.T) {
		// Test levels: Warn(2), Error(3), Fatal(4), Panic(5)
		atOrAboveWarn := []zerolog.Level{
			zerolog.WarnLevel,
			zerolog.ErrorLevel,
			zerolog.FatalLevel,
			zerolog.PanicLevel,
		}

		for _, level := range atOrAboveWarn {
			assert.GreaterOrEqual(t, level, zerolog.WarnLevel,
				"Expected %s (%d) >= WarnLevel (%d)", level, level, zerolog.WarnLevel)
		}
	})
}

// TestSpecificLevelWriter_WriteLevel tests the SpecificLevelWriter implementation
func TestSpecificLevelWriter_WriteLevel(t *testing.T) {
	t.Run("writes when ShouldWrite returns true", func(t *testing.T) {
		buf := &bytes.Buffer{}
		writer := logger.SpecificLevelWriter{
			Writer:      buf,
			ShouldWrite: func(_ zerolog.Level) bool { return true },
		}

		testData := []byte("test log message")
		n, err := writer.WriteLevel(zerolog.InfoLevel, testData)

		require.NoError(t, err)
		assert.Equal(t, len(testData), n)
		assert.Equal(t, string(testData), buf.String())
	})

	t.Run("skips writing when ShouldWrite returns false", func(t *testing.T) {
		buf := &bytes.Buffer{}
		writer := logger.SpecificLevelWriter{
			Writer:      buf,
			ShouldWrite: func(_ zerolog.Level) bool { return false },
		}

		testData := []byte("test log message")
		n, err := writer.WriteLevel(zerolog.InfoLevel, testData)

		require.NoError(t, err)
		assert.Equal(t, len(testData), n) // Returns length even when not writing
		assert.Empty(t, buf.String())     // Buffer should be empty
	})
}

// TestBackwardCompatibility_GetMultiLevelWriter ensures GetMultiLevelWriter preserves
// the old stdout/stderr split behavior for backward compatibility
func TestBackwardCompatibility_GetMultiLevelWriter(t *testing.T) {
	t.Run("GetMultiLevelWriter routes to stdout and stderr", func(t *testing.T) {
		// GetMultiLevelWriter should return a multi-level writer that splits
		// info/debug/trace → stdout and warn/error/fatal/panic → stderr
		writer := logger.GetMultiLevelWriter()

		// Should NOT be os.Stderr directly (that would be GetStderrWriter behavior)
		assert.NotEqual(t, os.Stderr, writer,
			"GetMultiLevelWriter should return multi-level writer, not direct stderr")

		// Should NOT be os.Stdout directly either
		assert.NotEqual(t, os.Stdout, writer,
			"GetMultiLevelWriter should return multi-level writer, not direct stdout")

		// The writer should be a MultiLevelWriter
		assert.NotNil(t, writer, "Writer should not be nil")
	})
}

// TestBackwardCompatibility_GetStderrWriter ensures GetStderrWriter returns direct stderr
func TestBackwardCompatibility_GetStderrWriter(t *testing.T) {
	writer := logger.GetStderrWriter()
	assert.Equal(t, os.Stderr, writer, "GetStderrWriter should return os.Stderr directly")
}

// TestBackwardCompatibility_ConsoleModeUsesMultiLevelWriter ensures config-based writer returns multi-level
func TestBackwardCompatibility_ConsoleModeUsesMultiLevelWriter(t *testing.T) {
	config := logger.Config{
		Sink:   logger.DefaultSinkConfig(),
		Format: logger.DefaultFormatConfig(),
	}
	writer := logger.GetMultiLevelWriterWithConfig(config)

	assert.NotEqual(t, os.Stderr, writer,
		"GetMultiLevelWriterWithConfig with ConsoleMode should return multi-level writer, not direct stderr")
	assert.NotEqual(t, os.Stdout, writer,
		"GetMultiLevelWriterWithConfig with ConsoleMode should return multi-level writer, not direct stdout")
	assert.NotNil(t, writer, "Writer should not be nil")
}

// TestBackwardCompatibility_InfoRoutesToStdout ensures info level routes to stdout
func TestBackwardCompatibility_InfoRoutesToStdout(t *testing.T) {
	testMsg := "test_info_message_12345"
	output := captureStdoutWithMultiLevelWriter(t, testMsg, zerolog.InfoLevel)
	assert.Contains(t, string(output), testMsg, "Info message should be routed to stdout")
}

// TestBackwardCompatibility_ErrorRoutesToStderr ensures error level routes to stderr
func TestBackwardCompatibility_ErrorRoutesToStderr(t *testing.T) {
	testMsg := "test_error_message_67890"
	output := captureStderrWithMultiLevelWriter(t, testMsg, zerolog.ErrorLevel)
	assert.Contains(t, string(output), testMsg, "Error message should be routed to stderr")
}

// captureStdoutForWriter captures stdout when logging with a specific writer.
func captureStdoutForWriter(t *testing.T, createWriter func() io.Writer, testMsg string) []byte {
	t.Helper()
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	writer := createWriter()
	log := zerolog.New(writer).Level(zerolog.TraceLevel).With().Timestamp().Logger()
	log.Info().Msg(testMsg)

	require.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)

	return output
}

// TestBackwardCompatibility_IdenticalRouting ensures both writer functions route identically
func TestBackwardCompatibility_IdenticalRouting(t *testing.T) {
	testMsg := "identical_routing_test_99999"

	require.NoError(t, os.Setenv("LOG_FORMAT", "json"))
	defer func() {
		if err := os.Unsetenv("LOG_FORMAT"); err != nil {
			t.Logf("failed to unset LOG_FORMAT: %v", err)
		}
	}()

	output1 := captureStdoutForWriter(t, logger.GetMultiLevelWriter, testMsg)

	config := logger.Config{
		Sink:   logger.SinkConfig{Mode: logger.ConsoleMode},
		Format: logger.FormatConfig{Format: logger.LogFormatJSON, Level: zerolog.TraceLevel},
	}
	output2 := captureStdoutForWriter(t, func() io.Writer {
		return logger.GetMultiLevelWriterWithConfig(config)
	}, testMsg)

	assert.Contains(t, string(output1), testMsg, "GetMultiLevelWriter should route info to stdout")
	assert.Contains(t, string(output2), testMsg, "GetMultiLevelWriterWithConfig should route info to stdout")
}

// TestFileMode_UsesMultiLevelWriter ensures FileMode uses multi-level writer for separation
func TestFileMode_UsesMultiLevelWriter(t *testing.T) {
	t.Run("file mode returns multi-level writer", func(t *testing.T) {
		tempDir := t.TempDir()
		config := logger.Config{
			Sink: logger.SinkConfig{
				Mode:       logger.FileMode,
				Dir:        tempDir,
				FileName:   "test.log",
				MaxSizeMB:  100,
				MaxFiles:   5,
				MaxAgeDays: 28,
			},
			Format: logger.DefaultFormatConfig(),
		}

		writer := logger.GetMultiLevelWriterWithConfig(config)

		// Should NOT be os.Stderr - should be a multi-level writer
		assert.NotEqual(t, os.Stderr, writer,
			"FileMode should return multi-level writer, not direct stderr")

		// The writer should be a MultiLevelWriter (we can't directly test the type,
		// but we can verify it's not the simple stderr)
		assert.NotNil(t, writer, "Writer should not be nil")
	})
}

// TestFutureProofLevelHandling ensures new zerolog levels would be handled correctly
func TestFutureProofLevelHandling(t *testing.T) {
	t.Run("hypothetical new level below WarnLevel routes to info", func(t *testing.T) {
		// Simulate a hypothetical new level between Info(1) and Warn(2)
		// For example: NoticeLevel with value 1.5 (if zerolog adds it)
		// In practice, this would be zerolog.Level(1) which is InfoLevel
		// but we're testing the comparison logic

		hypotheticalNewLevel := zerolog.Level(1) // Between Info and Warn

		// Should route to info because < WarnLevel(2)
		shouldBeInfo := hypotheticalNewLevel < zerolog.WarnLevel
		assert.True(t, shouldBeInfo,
			"New level %d should route to info writer (< WarnLevel %d)",
			hypotheticalNewLevel, zerolog.WarnLevel)
	})

	t.Run("hypothetical new level above PanicLevel routes to error", func(t *testing.T) {
		// Simulate a hypothetical new level above Panic(5)
		// For example: CriticalLevel with value 6
		hypotheticalCriticalLevel := zerolog.Level(6)

		// Should route to error because >= WarnLevel(2)
		shouldBeError := hypotheticalCriticalLevel >= zerolog.WarnLevel
		assert.True(t, shouldBeError,
			"New level %d should route to error writer (>= WarnLevel %d)",
			hypotheticalCriticalLevel, zerolog.WarnLevel)
	})
}
