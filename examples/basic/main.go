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

func main() {
	ctx := context.Background()

	m := map[string]any{
		"app":         "basic-logspan-example",
		"test-name":   "basic",
		"int-value":   1,
		"bool-value":  true,
		"float-value": 30.5,
	}

	rootKeysAndValues := []any{}
	for k, v := range m {
		rootKeysAndValues = append(rootKeysAndValues, k, v)
	}

	// Initialize logger with environment configuration and store in context
	logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides()).
		WithName("BasicExample").
		WithValues(rootKeysAndValues...)
	ctx = logger.ContextWithLogr(ctx, logr)
	ctxLogger := logger.MustLogrFromContext(ctx)

	// Setup OpenTelemetry SDK with custom resource attributes
	// Resource attributes are metadata attached to all spans from this service
	//
	// API Option 1: SetupOTelSDKFrom - Explicit config with env override
	// This follows the same pattern as logger.CreateLoggerFrom
	// config := instrumentation.OTelConfig{
	//     Endpoint: "http://localhost:4317",  // DEFAULT value
	// }
	// config = instrumentation.NewOTelConfigFromEnv(config)  // Env can override
	// shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "basic-logspan-example", "v1.0.0", ctxLogger, config, rootKeysAndValues...)
	//
	// API Option 2: SetupOTelSDKWithOptions - Functional options (env overrides)
	// This follows the same pattern as logger.CreateLogger
	// Options set DEFAULT values that can be overridden by OTEL_EXPORTER_OTLP_ENDPOINT env var
	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
		ctx,
		"basic-logspan-example",
		"v1.0.0",
		ctxLogger,
		// WithDefaultOTLPEndpoint sets DEFAULT that OTEL_EXPORTER_OTLP_ENDPOINT env can override
		// instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
		instrumentation.WithResourceAttributes(rootKeysAndValues...),
	)
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

	logger.Info("innerFunc1 is called", "func", "innerFunc1")
}

func innerFunc2(ctx context.Context) {
	_, logger, end := instrumentation.GetLogSpan(ctx, "innerFunc2")
	defer end()

	logger.Info("innerFunc2 is called", "func", "innerFunc2")
}
