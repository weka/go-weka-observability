//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"

	zerologger "github.com/weka/go-weka-observability/logger"
)

func main() {
	fmt.Println("=== Testing Logger Modes ===")

	// Test 1: File mode using functional options (NEW API)
	fmt.Println("\n1. Testing file mode with functional options:")
	config := zerologger.Config{
		Sink: zerologger.SinkConfig{
			Mode:       zerologger.FileMode,
			Dir:        "test-logs",
			FileName:   "tg-cli.log",
			MaxSizeMB:  100,
			MaxFiles:   5,
			MaxAgeDays: 28,
		},
		Format: zerologger.FormatConfig{
			Level:        zerolog.InfoLevel,
			Format:       zerologger.LogFormatJSON,
			CallerDirLvl: -1,
		},
	}
	logger := zerologger.NewZeroLoggerWithConfig(config)

	logger.Info().Msg("This is an info message in CLI mode")
	logger.Warn().Msg("This is a warning message in CLI mode")
	logger.Error().Msg("This is an error message in CLI mode")

	// Test 2: Console mode (default)
	fmt.Println("\n2. Testing console mode:")
	logger2 := zerologger.NewZeroLoggerWithConfig(zerologger.DefaultConfig())

	logger2.Info().Msg("This is an info message in console mode")
	logger2.Warn().Msg("This is a warning message in console mode")
	logger2.Error().Msg("This is an error message in console mode")

	// Test 3: Environment variable override
	fmt.Println("\n3. Testing environment variable override:")
	os.Setenv("LOG_MODE", "file")
	os.Setenv("LOG_DIR", "./test-logs")
	os.Setenv("LOG_FILE_NAME", "test.log")

	envConfig := zerologger.NewDefaultConfigWithEnvOverrides()
	logger3 := zerologger.NewZeroLoggerWithConfig(envConfig)

	logger3.Info().Msg("This message should go to test-logs/test.log")
	logger3.Warn().Msg("This warning should go to test-logs/test-error.log")

	fmt.Println("\n=== Test completed ===")
	fmt.Println("Check the following files for output:")
	fmt.Println("- test-logs/tg-cli.log (file mode info logs)")
	fmt.Println("- test-logs/tg-cli-error.log (file mode error logs)")
	fmt.Println("- test-logs/test.log (env override info logs)")
	fmt.Println("- test-logs/test-error.log (env override error logs)")
}
