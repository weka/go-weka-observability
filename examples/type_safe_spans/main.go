package main

import (
	"context"
	"fmt"
	"time"

	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Example demonstrating type-safe span creation with OpenTelemetry options.
// This example shows how to use CreateSpanWithOptions, CreateRootSpanWithOptions,
// and convenience functions for common span kinds.
func main() {
	ctx := context.Background()

	// 1. Setup logger
	logr := logger.CreateLogger(
		logger.WithConsoleSink(),
		logger.WithInfoLevel(),
	)
	ctx = logger.ContextWithLogr(ctx, logr)

	// 2. Setup OpenTelemetry
	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
		ctx, "type-safe-spans-example", "v1.0.0", logr,
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to setup OTel: %v", err))
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			logr.Error(err, "Failed to shutdown OTel")
		}
	}()

	// 3. Demonstrate different span creation patterns
	demonstrateTypeSefeSpanCreation(ctx)
}

func demonstrateTypeSefeSpanCreation(ctx context.Context) {
	// Example 1: CreateSpanWithOptions for type-safe span creation
	ctx, dbLogger := instrumentation.CreateSpanWithOptions(ctx, "database-query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.name", "users_db"),
			attribute.String("db.statement", "SELECT * FROM users WHERE active = true"),
		),
	)
	defer dbLogger.End()

	dbLogger.Info("Executing database query")
	simulateWork()
	dbLogger.SetAttributes(attribute.Int("db.rows_returned", 42))
	dbLogger.Info("Query completed successfully")

	// Example 2: Convenience functions for common span kinds
	demonstrateConvenienceFunctions(ctx)

	// Example 3: CreateRootSpanWithOptions for independent traces
	demonstrateRootSpanWithOptions(ctx)

	// Example 4: Advanced - GetTracer for direct tracer access
	demonstrateDirectTracerAccess(ctx)
}

func demonstrateConvenienceFunctions(ctx context.Context) {
	// Server span - for HTTP/gRPC request handlers
	ctx, serverLogger := instrumentation.CreateServerSpan(ctx, "http-request-handler",
		trace.WithAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.route", "/api/users"),
			attribute.String("http.scheme", "https"),
		),
	)
	defer serverLogger.End()

	serverLogger.Info("Processing incoming HTTP request")
	simulateWork()

	// Client span - for outgoing HTTP/gRPC calls
	ctx, clientLogger := instrumentation.CreateClientSpan(ctx, "external-api-call",
		trace.WithAttributes(
			attribute.String("http.method", "POST"),
			attribute.String("http.url", "https://api.example.com/data"),
			attribute.Int("http.status_code", 200),
		),
	)
	defer clientLogger.End()

	clientLogger.Info("Making external API call")
	simulateWork()
	clientLogger.Info("API call completed")

	// Producer span - for message publishing
	ctx, producerLogger := instrumentation.CreateProducerSpan(ctx, "publish-event",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", "user-events"),
			attribute.String("messaging.operation", "publish"),
		),
	)
	defer producerLogger.End()

	producerLogger.Info("Publishing message to Kafka")
	simulateWork()
	producerLogger.SetAttributes(attribute.String("messaging.message_id", "msg-12345"))
	producerLogger.Info("Message published successfully")

	// Consumer span - for message consumption
	consumerCtx, consumerLogger := instrumentation.CreateConsumerSpan(ctx, "consume-event",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", "user-events"),
			attribute.String("messaging.operation", "receive"),
		),
	)
	defer consumerLogger.End()

	consumerLogger.Info("Processing message from Kafka")
	processMessage(consumerCtx)
	consumerLogger.Info("Message processed successfully")
}

func demonstrateRootSpanWithOptions(ctx context.Context) {
	// CreateRootSpanWithOptions creates a new independent trace
	// (breaks parent span relationship, starts new trace ID)
	_, jobLogger := instrumentation.CreateRootSpanWithOptions(ctx, "background-cleanup-job",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("job.type", "cleanup"),
			attribute.String("job.schedule", "daily"),
		),
	)
	defer jobLogger.End()

	jobLogger.Info("Starting background cleanup job")
	simulateWork()
	jobLogger.SetAttributes(attribute.Int("records.deleted", 150))
	jobLogger.Info("Cleanup job completed")
}

func demonstrateDirectTracerAccess(ctx context.Context) {
	// Advanced: GetTracer for direct tracer access
	// Use this when you need to integrate with third-party OTel libraries
	// or have specialized span creation requirements
	tracer := instrumentation.GetTracer(ctx)

	ctx, span := tracer.Start(ctx, "custom-span-with-raw-tracer",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("custom.field", "value"),
		),
	)
	defer span.End()

	// Note: No SpanLogger integration - manual span and logging management
	span.SetAttributes(attribute.Bool("operation.success", true))

	// You can still get a logger from context for logging
	logger := logger.MustLogrFromContext(ctx)
	logger.Info("Custom span with direct tracer access")

	simulateWork()
}

func processMessage(ctx context.Context) {
	// Helper function that logs under current span (no new span created)
	view := instrumentation.CurrentSpanLogger(ctx)
	view.Info("Validating message")
	simulateWork()
	view.Info("Message validation completed")
	// Note: No End() call - we don't own the span
}

func simulateWork() {
	time.Sleep(50 * time.Millisecond)
}
