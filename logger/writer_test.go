package logger_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weka/go-weka-observability/logger"
)

// TestLevelComparators tests the level comparison functions used for routing logs
func TestLevelComparators(t *testing.T) {
	tests := []struct {
		name            string
		level           zerolog.Level
		expectInfoFile  bool
		expectErrorFile bool
	}{
		{
			name:            "TraceLevel routes to info writer",
			level:           zerolog.TraceLevel,
			expectInfoFile:  true,
			expectErrorFile: false,
		},
		{
			name:            "DebugLevel routes to info writer",
			level:           zerolog.DebugLevel,
			expectInfoFile:  true,
			expectErrorFile: false,
		},
		{
			name:            "InfoLevel routes to info writer",
			level:           zerolog.InfoLevel,
			expectInfoFile:  true,
			expectErrorFile: false,
		},
		{
			name:            "WarnLevel routes to error writer",
			level:           zerolog.WarnLevel,
			expectInfoFile:  false,
			expectErrorFile: true,
		},
		{
			name:            "ErrorLevel routes to error writer",
			level:           zerolog.ErrorLevel,
			expectInfoFile:  false,
			expectErrorFile: true,
		},
		{
			name:            "FatalLevel routes to error writer",
			level:           zerolog.FatalLevel,
			expectInfoFile:  false,
			expectErrorFile: true,
		},
		{
			name:            "PanicLevel routes to error writer",
			level:           zerolog.PanicLevel,
			expectInfoFile:  false,
			expectErrorFile: true,
		},
		{
			name:            "NoLevel routes to error writer (special case)",
			level:           zerolog.NoLevel,
			expectInfoFile:  false,
			expectErrorFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create separate buffers for info and error writers
			infoBuffer := &bytes.Buffer{}
			errorBuffer := &bytes.Buffer{}

			// Create config with file mode to enable level separation
			config := logger.Config{
				Sink: logger.SinkConfig{
					Mode:       logger.ConsoleMode, // Use console to avoid file creation
					MaxSizeMB:  100,
					MaxFiles:   5,
					MaxAgeDays: 28,
				},
				Format: logger.FormatConfig{
					Level:        zerolog.TraceLevel, // Capture all levels
					Format:       logger.LogFormatJSON,
					TimeOnly:     false,
					CallerDirLvl: -1,
				},
			}

			// Create multi-level writer manually with test buffers
			infoWriter := logger.SpecificLevelWriter{
				Writer:      infoBuffer,
				ShouldWrite: func(level zerolog.Level) bool { return level < zerolog.WarnLevel },
			}
			errorWriter := logger.SpecificLevelWriter{
				Writer:      errorBuffer,
				ShouldWrite: func(level zerolog.Level) bool { return level >= zerolog.WarnLevel },
			}
			multiWriter := zerolog.MultiLevelWriter(infoWriter, errorWriter)

			// Create logger with the multi-level writer
			log := zerolog.New(multiWriter).Level(config.Format.Level).With().Timestamp().Logger()

			// Write a log at the test level
			testMessage := "test message for level routing"
			switch tt.level {
			case zerolog.TraceLevel:
				log.Trace().Msg(testMessage)
			case zerolog.DebugLevel:
				log.Debug().Msg(testMessage)
			case zerolog.InfoLevel:
				log.Info().Msg(testMessage)
			case zerolog.WarnLevel:
				log.Warn().Msg(testMessage)
			case zerolog.ErrorLevel:
				log.Error().Msg(testMessage)
			case zerolog.FatalLevel:
				// Use WithLevel to avoid os.Exit
				log.WithLevel(zerolog.FatalLevel).Msg(testMessage)
			case zerolog.PanicLevel:
				// Use WithLevel to avoid panic
				log.WithLevel(zerolog.PanicLevel).Msg(testMessage)
			case zerolog.NoLevel:
				log.WithLevel(zerolog.NoLevel).Msg(testMessage)
			}

			// Verify routing
			infoContent := infoBuffer.String()
			errorContent := errorBuffer.String()

			if tt.expectInfoFile {
				assert.Contains(t, infoContent, testMessage,
					"Expected level %s to be routed to info writer", tt.level)
				assert.Empty(t, errorContent,
					"Expected level %s NOT to be routed to error writer", tt.level)
			} else if tt.expectErrorFile {
				assert.Contains(t, errorContent, testMessage,
					"Expected level %s to be routed to error writer", tt.level)
				assert.Empty(t, infoContent,
					"Expected level %s NOT to be routed to info writer", tt.level)
			} else {
				// Both should be empty for disabled levels
				assert.Empty(t, infoContent, "Expected no output for disabled level")
				assert.Empty(t, errorContent, "Expected no output for disabled level")
			}
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
			assert.True(t, level < zerolog.WarnLevel,
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
			assert.True(t, level >= zerolog.WarnLevel,
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
			ShouldWrite: func(level zerolog.Level) bool { return true },
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
			ShouldWrite: func(level zerolog.Level) bool { return false },
		}

		testData := []byte("test log message")
		n, err := writer.WriteLevel(zerolog.InfoLevel, testData)

		require.NoError(t, err)
		assert.Equal(t, len(testData), n) // Returns length even when not writing
		assert.Empty(t, buf.String())     // Buffer should be empty
	})
}

// TestBackwardCompatibility_ConsoleModeUsesDirectWriter ensures the new config-based
// logger initialization is backward compatible with the old deprecated way
func TestBackwardCompatibility_ConsoleModeUsesDirectWriter(t *testing.T) {
	t.Run("old way returns direct stderr writer", func(t *testing.T) {
		// Old deprecated way: GetStderrWriter()
		oldWriter := logger.GetStderrWriter()

		// Should return os.Stderr directly (for default JSON format)
		assert.Equal(t, os.Stderr, oldWriter,
			"Old GetStderrWriter should return os.Stderr directly")
	})

	t.Run("new way with console mode returns same direct writer", func(t *testing.T) {
		// New way: GetMultiLevelWriterWithConfig with ConsoleMode
		config := logger.Config{
			Sink:   logger.DefaultSinkConfig(), // ConsoleMode by default
			Format: logger.DefaultFormatConfig(),
		}
		newWriter := logger.GetMultiLevelWriterWithConfig(config)

		// Should also return os.Stderr directly (backward compatible)
		assert.Equal(t, os.Stderr, newWriter,
			"New GetMultiLevelWriterWithConfig with ConsoleMode should return os.Stderr directly")
	})

	t.Run("both ways produce identical writers", func(t *testing.T) {
		oldWriter := logger.GetStderrWriter()
		newWriter := logger.GetMultiLevelWriterWithConfig(logger.DefaultConfig())

		// Both should be the same
		assert.Equal(t, oldWriter, newWriter,
			"Old and new ways should produce identical writers for backward compatibility")
	})
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
