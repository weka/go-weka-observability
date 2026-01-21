package main

import (
	"context"
	"errors"
	"os"

	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/logger"
)

func init() {
	// Set default log level and format via environment variables
	// These are automatically picked up by NewDefaultConfigWithEnvOverrides()
	if os.Getenv("LOG_LEVEL") == "" {
		_ = os.Setenv("LOG_LEVEL", "0")
	}
	if os.Getenv("LOG_FORMAT") == "" {
		_ = os.Setenv("LOG_FORMAT", "raw")
	}
	if os.Getenv("LOG_CALLER_DIR_LVL") == "" {
		_ = os.Setenv("LOG_CALLER_DIR_LVL", "1")
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

	// Initialize logger: console sink, raw format (with colors), debug level
	// Functional options set defaults, but env vars from init() override them
	logr := logger.CreateLogger(
		logger.WithConsoleSink(),
		logger.WithRawFormat(),
		logger.WithDebugLevel(),
	)

	logr = logr.WithName("BasicExample").
		WithValues(rootKeysAndValues...)

	ctx = logger.ContextWithLogr(ctx, logr)

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
	// API Option 2: SetupOTelSDKWithOptions - Functional options (env always takes precedence)
	// This follows the same pattern as logger.CreateLogger
	//
	// OTEL_EXPORTER_OTLP_ENDPOINT environment variable always takes precedence if set,
	// regardless of whether you use WithDefaultOTLPEndpoint or not.
	//
	// Note: If no collector is running at the endpoint, traces won't be exported but the
	// example will still run successfully (graceful degradation)
	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
		ctx,
		"basic-logspan-example",
		"v1.0.0",
		logr,
		// WithDefaultOTLPEndpoint sets fallback endpoint when OTEL_EXPORTER_OTLP_ENDPOINT is not set
		instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
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
	// CreateLogSpan creates a new child span that you own
	// You MUST call defer logger.End() to properly close the span
	ctx, logger := instrumentation.CreateLogSpan(ctx, "outerFunc")
	defer logger.End()

	logger.Info("outerFunc is called")

	innerFunc1(ctx)
	innerFunc2(ctx)
}

func innerFunc1(ctx context.Context) {
	// CreateLogSpan creates a new child span (child of outerFunc's span)
	// The span is automatically linked to its parent through the context
	_, logger := instrumentation.CreateLogSpan(ctx, "innerFunc1")
	defer logger.End()

	logger.Info("innerFunc1 is called", "func", "innerFunc1")
}

func innerFunc2(ctx context.Context) {
	// CreateLogSpan creates a new child span
	_, logger := instrumentation.CreateLogSpan(ctx, "innerFunc2")
	defer logger.End()

	logger.Info("innerFunc2 is called", "func", "innerFunc2")

	err := errors.New("debug")
	// SetError logs the error AND marks the span with Error status in OpenTelemetry.
	// This makes the span appear as failed in tracing UIs (e.g., Signoz, Jaeger, Grafana Tempo).
	// Use SetError() when the error represents a failure of the operation.
	// Use Error() when the error is recoverable and the operation succeeds overall.
	logger.SetError(err, "debug error occurred in innerFunc2", "additional-info", 12345)
}
