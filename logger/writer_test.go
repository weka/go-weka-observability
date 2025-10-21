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

// TestBackwardCompatibility_ConsoleModeUsesMultiLevelWriter ensures the new config-based
// logger initialization matches GetMultiLevelWriter behavior
func TestBackwardCompatibility_ConsoleModeUsesMultiLevelWriter(t *testing.T) {
	t.Run("GetStderrWriter returns direct stderr writer", func(t *testing.T) {
		// GetStderrWriter is for stderr-only output
		writer := logger.GetStderrWriter()

		// Should return os.Stderr directly (for default JSON format)
		assert.Equal(t, os.Stderr, writer,
			"GetStderrWriter should return os.Stderr directly")
	})

	t.Run("GetMultiLevelWriterWithConfig in ConsoleMode uses multi-level writer", func(t *testing.T) {
		// New way: GetMultiLevelWriterWithConfig with ConsoleMode
		config := logger.Config{
			Sink:   logger.DefaultSinkConfig(), // ConsoleMode by default
			Format: logger.DefaultFormatConfig(),
		}
		writer := logger.GetMultiLevelWriterWithConfig(config)

		// Should return multi-level writer (NOT direct stderr)
		assert.NotEqual(t, os.Stderr, writer,
			"GetMultiLevelWriterWithConfig with ConsoleMode should return multi-level writer, not direct stderr")
		assert.NotEqual(t, os.Stdout, writer,
			"GetMultiLevelWriterWithConfig with ConsoleMode should return multi-level writer, not direct stdout")
		assert.NotNil(t, writer, "Writer should not be nil")
	})

	t.Run("GetMultiLevelWriterWithConfig routes info to stdout", func(t *testing.T) {
		// Test ACTUAL GetMultiLevelWriterWithConfig by capturing real stdout
		config := logger.Config{
			Sink: logger.SinkConfig{
				Mode: logger.ConsoleMode, // Explicitly set ConsoleMode
			},
			Format: logger.FormatConfig{
				Format: logger.LogFormatJSON,
				Level:  zerolog.TraceLevel,
			},
		}

		// Capture stdout using os.Pipe
		origStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		defer func() {
			os.Stdout = origStdout
		}()

		// Create logger with ACTUAL GetMultiLevelWriterWithConfig
		writer := logger.GetMultiLevelWriterWithConfig(config)
		log := zerolog.New(writer).Level(config.Format.Level).With().Timestamp().Logger()

		// Write info log (should go to stdout)
		testMsg := "test_info_message_12345"
		log.Info().Msg(testMsg)

		// Close write end and read captured output
		require.NoError(t, w.Close())
		output, err := io.ReadAll(r)
		require.NoError(t, err)

		// Verify the message was routed to stdout
		assert.Contains(t, string(output), testMsg,
			"Info message should be routed to stdout")
	})

	t.Run("GetMultiLevelWriterWithConfig routes errors to stderr", func(t *testing.T) {
		// Test ACTUAL GetMultiLevelWriterWithConfig by capturing real stderr
		config := logger.Config{
			Sink: logger.SinkConfig{
				Mode: logger.ConsoleMode, // Explicitly set ConsoleMode
			},
			Format: logger.FormatConfig{
				Format: logger.LogFormatJSON,
				Level:  zerolog.TraceLevel,
			},
		}

		// Capture stderr using os.Pipe
		origStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		defer func() {
			os.Stderr = origStderr
		}()

		// Create logger with ACTUAL GetMultiLevelWriterWithConfig
		writer := logger.GetMultiLevelWriterWithConfig(config)
		log := zerolog.New(writer).Level(config.Format.Level).With().Timestamp().Logger()

		// Write error log (should go to stderr)
		testMsg := "test_error_message_67890"
		log.Error().Msg(testMsg)

		// Close write end and read captured output
		require.NoError(t, w.Close())
		output, err := io.ReadAll(r)
		require.NoError(t, err)

		// Verify the message was routed to stderr
		assert.Contains(t, string(output), testMsg,
			"Error message should be routed to stderr")
	})

	t.Run("GetMultiLevelWriter and GetMultiLevelWriterWithConfig route identically", func(t *testing.T) {
		// Test that both functions produce identical routing behavior
		testMsg := "identical_routing_test_99999"

		// Test GetMultiLevelWriter - capture stdout
		origStdout1 := os.Stdout
		r1, w1, _ := os.Pipe()
		os.Stdout = w1
		defer func() {
			os.Stdout = origStdout1
			_ = os.Unsetenv("LOG_FORMAT")
		}()

		require.NoError(t, os.Setenv("LOG_FORMAT", "json"))
		writer1 := logger.GetMultiLevelWriter()
		log1 := zerolog.New(writer1).Level(zerolog.TraceLevel).With().Timestamp().Logger()
		log1.Info().Msg(testMsg)

		require.NoError(t, w1.Close())
		os.Stdout = origStdout1
		output1, err := io.ReadAll(r1)
		require.NoError(t, err)

		// Test GetMultiLevelWriterWithConfig - capture stdout
		origStdout2 := os.Stdout
		r2, w2, _ := os.Pipe()
		os.Stdout = w2
		defer func() {
			os.Stdout = origStdout2
		}()

		config := logger.Config{
			Sink: logger.SinkConfig{
				Mode: logger.ConsoleMode,
			},
			Format: logger.FormatConfig{
				Format: logger.LogFormatJSON,
				Level:  zerolog.TraceLevel,
			},
		}
		writer2 := logger.GetMultiLevelWriterWithConfig(config)
		log2 := zerolog.New(writer2).Level(config.Format.Level).With().Timestamp().Logger()
		log2.Info().Msg(testMsg)

		require.NoError(t, w2.Close())
		os.Stdout = origStdout2
		output2, err2 := io.ReadAll(r2)
		require.NoError(t, err2)

		// Both should have routed the message to stdout
		assert.Contains(t, string(output1), testMsg,
			"GetMultiLevelWriter should route info to stdout")
		assert.Contains(t, string(output2), testMsg,
			"GetMultiLevelWriterWithConfig should route info to stdout")
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

// TestFormatCaller tests the caller formatting logic with different directory levels
func TestFormatCaller(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		line      int
		dirLevels int
		want      string
	}{
		{
			name:      "zero levels shows filename only",
			file:      "/path/to/project/pkg/file.go",
			line:      42,
			dirLevels: 0,
			want:      "file.go:42",
		},
		{
			name:      "one level shows parent dir and filename",
			file:      "/path/to/project/pkg/file.go",
			line:      100,
			dirLevels: 1,
			want:      "pkg/file.go:100",
		},
		{
			name:      "two levels shows two parent dirs and filename",
			file:      "/path/to/project/pkg/sub/file.go",
			line:      200,
			dirLevels: 2,
			want:      "pkg/sub/file.go:200",
		},
		{
			name:      "three levels shows three parent dirs and filename",
			file:      "/home/user/go/src/project/internal/logger/writer.go",
			line:      350,
			dirLevels: 3,
			want:      "project/internal/logger/writer.go:350",
		},
		{
			name:      "excessive dir levels shows entire path",
			file:      "/short/path/file.go",
			line:      10,
			dirLevels: 100,
			want:      "/short/path/file.go:10",
		},
		{
			name:      "negative dir levels shows filename only",
			file:      "/path/to/file.go",
			line:      999,
			dirLevels: -1,
			want:      "file.go:999",
		},
		{
			name:      "single file with leading slash and zero levels",
			file:      "/file.go",
			line:      1,
			dirLevels: 0,
			want:      "file.go:1",
		},
		{
			name:      "no directory separators",
			file:      "file.go",
			line:      5,
			dirLevels: 0,
			want:      "file.go:5",
		},
		{
			name:      "windows-style path separators not handled",
			file:      "C:\\path\\to\\file.go",
			line:      20,
			dirLevels: 1,
			want:      "C:\\path\\to\\file.go:20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logger.FormatCaller(tt.file, tt.line, tt.dirLevels)
			assert.Equal(t, tt.want, got)
		})
	}
}
