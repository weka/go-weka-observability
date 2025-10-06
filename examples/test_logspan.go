//go:build ignore
// +build ignore

package main

import (
	"context"
	"os"

	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/logger"
)

func init() {
	// Set default log level and format via environment variables
	// These are automatically picked up by NewDefaultConfigWithEnvOverrides()
	if os.Getenv("LOG_LEVEL") == "" {
		os.Setenv("LOG_LEVEL", "0")
	}
	if os.Getenv("LOG_FORMAT") == "" {
		os.Setenv("LOG_FORMAT", "raw")
	}
	if os.Getenv("LOG_CALLER_DIR_LVL") == "" {
		os.Setenv("LOG_CALLER_DIR_LVL", "1")
	}
}

// NOTE: set OTEL_EXPORTER_OTLP_ENDPOINT=https://otelcollector.rnd.weka.io:4317 environment variable before running this test
func TestLogSpan() {
	ctx := context.Background()

	// Initialize logger with environment configuration and store in context
	logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides()).WithName("Test")
	ctx = logger.ContextWithLogr(ctx, logr)
	ctxLogger := logger.MustLogrFromContext(ctx)

	shutdown, err := instrumentation.SetupOTelSDK(context.Background(), "test-logspan", "v0.0.1", ctxLogger)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := shutdown(ctx)
		if err != nil {
			panic(err)
		}
	}()
	outerFunc(ctx)
}

func outerFunc(ctx context.Context) {
	ctx, logger, end := instrumentation.GetLogSpan(ctx, "outerFunc")
	defer end()

	logger.Info("outerFunc is called")

	innerFunc1(ctx)
	innerFunc2(ctx)
}

func innerFunc1(ctx context.Context) {
	_, logger, end := instrumentation.GetLogSpan(ctx, "innerFunc1")
	defer end()

	logger.Info("innerFunc1 is called")
}

func innerFunc2(ctx context.Context) {
	_, logger, end := instrumentation.GetLogSpan(ctx, "innerFunc2")
	defer end()

	logger.Info("innerFunc2 is called")
}

func main() {
	TestLogSpan()
}
