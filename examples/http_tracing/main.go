package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/logger"
)

func init() {
	// set default log level and format
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

// main demonstrates end-to-end trace propagation between client and server
func main() {
	ctx := context.Background()

	// Initialize logger: console sink, raw format (with colors), debug level
	// Functional options set defaults, but env vars from init() override them
	logr := logger.CreateLogger(
		logger.WithConsoleSink(),
		logger.WithRawFormat(),
		logger.WithDebugLevel(),
	).WithName("HTTPTracingExample")

	ctx = logger.ContextWithLogr(ctx, logr)

	// Setup OpenTelemetry SDK with options
	//
	// OTEL_EXPORTER_OTLP_ENDPOINT environment variable always takes precedence if set,
	// regardless of whether you use WithDefaultOTLPEndpoint or not.
	//
	// Note: If no collector is running at the endpoint, traces won't be exported but the
	// example will still run successfully (graceful degradation)
	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
		ctx, "http-tracing-example", "v1.0.0", logr,
		// WithDefaultOTLPEndpoint sets fallback endpoint when OTEL_EXPORTER_OTLP_ENDPOINT is not set
		// Comment this out if you want to run without a collector
		instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
	)
	if err != nil {
		panic(err)
	}
	defer func() {
		if shutdownErr := shutdown(ctx); shutdownErr != nil {
			logr.Error(shutdownErr, "Failed to shutdown OTel SDK")
		}
	}()

	// Create and start HTTP server in goroutine
	server := NewHTTPServer("8080")
	go func() {
		if startErr := server.Start(); startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
			logr.Error(startErr, "HTTP server error")
		}
	}()

	// Give server time to start
	time.Sleep(1 * time.Second)

	// Create HTTP client
	client := NewHTTPClient("http://localhost:8080")

	// EXAMPLE: CreateRootLogSpan - Start a new independent trace for client workflow
	// This breaks the parent chain and creates a new trace ID
	// Use this for background jobs or operations that should be tracked independently
	ctx, rootLogger := instrumentation.CreateRootLogSpan(ctx, "client.workflow")
	defer rootLogger.End()

	rootLogger.Info("Starting client workflow demonstration with independent trace")

	// Demonstrate multiple HTTP calls within the same trace

	// 1. Health check - EXAMPLE: CreateLogSpan for child operation
	healthCheckCtx, healthLogger := instrumentation.CreateLogSpan(ctx, "workflow.health_check")
	healthResp, err := client.Get(healthCheckCtx, "/health")
	if err != nil {
		healthLogger.Error(err, "Health check failed")
	} else {
		healthLogger.Info("Health check successful", "server_trace_id", healthResp.TraceID)
	}
	healthLogger.End()

	// 2. Data retrieval - EXAMPLE: CreateLogSpan for child operation
	getDataCtx, dataLogger := instrumentation.CreateLogSpan(ctx, "workflow.get_data")
	dataResp, err := client.Get(getDataCtx, "/api/data")
	if err != nil {
		dataLogger.Error(err, "Data retrieval failed")
	} else {
		dataLogger.Info("Data retrieved successfully",
			"server_trace_id", dataResp.TraceID,
			"data", dataResp.Data)
	}
	dataLogger.End()

	// 3. Data processing - EXAMPLE: CreateLogSpan for child operation
	processDataCtx, processLogger := instrumentation.CreateLogSpan(ctx, "workflow.process_data")
	processResp, err := client.Post(processDataCtx, "/api/process", map[string]string{
		"input": "sample_data_for_processing",
	})
	if err != nil {
		processLogger.Error(err, "Data processing failed")
	} else {
		processLogger.Info("Data processing completed",
			"server_trace_id", processResp.TraceID,
			"result", processResp.Data)
	}
	processLogger.End()

	rootLogger.Info("Client workflow completed successfully")

	// Gracefully shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logr.Error(err, "Failed to shutdown server")
	}

	logr.Info("HTTP trace propagation example completed")
}
