package main

import (
	"context"
	"os"

	"github.com/go-logr/zerologr"

	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/logger"
)

func init() {
	// set default log level and format
	if os.Getenv("LOG_LEVEL") == "" {
		os.Setenv("LOG_LEVEL", "0")
	}
	if os.Getenv("LOG_FORMAT") == "" {
		os.Setenv("LOG_FORMAT", "raw")
	}
	if os.Getenv("LOG_CALLER_DIR_LVL") == "" {
		os.Setenv("LOG_CALLER_DIR_LVL", "1")
	}

	logger.SetCallerDirDisplayLevel()
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

	// initialize root logger and put it into context
	logr := zerologr.New(logger.NewZeroLogger())
	ctx, ctxLogger := instrumentation.GetLoggerForContext(ctx, &logr, "BasicExample", rootKeysAndValues...)

	// Setup OpenTelemetry SDK with custom resource attributes
	// Resource attributes are metadata attached to all spans from this service
	shutdown, err := instrumentation.SetupOTelSDK(
		ctx,
		"basic-logspan-example",
		"v1.0.0",
		ctxLogger,
		rootKeysAndValues...,
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
