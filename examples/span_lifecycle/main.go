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

	// Initialize logger
	logr := logger.CreateLogger(
		logger.WithConsoleSink(),
		logger.WithRawFormat(),
		logger.WithDebugLevel(),
	).WithName("SpanLifecycleExample")
	ctx = logger.ContextWithLogr(ctx, logr)

	// Setup OpenTelemetry SDK
	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
		ctx,
		"span-lifecycle-example",
		"v1.0.0",
		logr,
	)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			panic(err)
		}
	}()

	logr.Info("=== Span Lifecycle Example: Demonstrating All Three API Functions ===")

	// Section 1: CreateSpan() - Creating owned spans
	demonstrateCreateSpan(ctx)

	// Section 2: CurrentSpanLogger() - Borrowing current span
	demonstrateCurrentSpanLogger(ctx)

	// Section 3: CreateRootSpan() - Breaking parent chain
	demonstrateCreateRootSpan(ctx)
}

// ============================================================================
// Section 1: CreateSpan() - Creating Owned Spans
// ============================================================================
//
// Use CreateSpan when:
// - You're creating a new operation that should be a child of the current span
// - You need to track the start and end of an operation
// - You want to add operation-specific logging and attributes
//
// Key Points:
// - Returns SpanLogger that you OWN
// - You MUST call defer logger.End()
// - New span becomes child of current span in context
// - All logs are automatically enriched with trace/span IDs

func demonstrateCreateSpan(ctx context.Context) {
	// Create a parent span for a user request
	ctx, parentLogger := instrumentation.CreateSpan(ctx, "process_user_request", "user_id", 12345)
	defer parentLogger.End() // MUST call - you own this span!

	parentLogger.Info("Processing user request started")

	// Create child spans for different operations
	validateUser(ctx)
	fetchUserData(ctx)
	processUserData(ctx)

	parentLogger.Info("Processing user request completed")
}

func validateUser(ctx context.Context) {
	// Child span 1: User validation
	ctx, logger := instrumentation.CreateSpan(ctx, "validate_user")
	defer logger.End()

	logger.Info("Validating user credentials")
	logger.Debug("Checking user permissions", "role", "admin")
	logger.Info("User validation successful")
}

func fetchUserData(ctx context.Context) {
	// Child span 2: Database query
	ctx, logger := instrumentation.CreateSpan(ctx, "fetch_user_data", "query", "SELECT * FROM users")
	defer logger.End()

	logger.Info("Querying database for user data")
	logger.Debug("Database connection established", "host", "db.example.com")
	logger.Info("User data retrieved successfully", "rows_returned", 5)
}

func processUserData(ctx context.Context) {
	// Child span 3: Data processing
	ctx, logger := instrumentation.CreateSpan(ctx, "process_user_data")
	defer logger.End()

	logger.Info("Processing user data")

	// Nested child span: Data transformation
	ctx, transformLogger := instrumentation.CreateSpan(ctx, "transform_data")
	defer transformLogger.End()

	transformLogger.Info("Transforming data format", "from", "JSON", "to", "Protobuf")
	transformLogger.Info("Data transformation complete")
}

// ============================================================================
// Section 2: CurrentSpanLogger() - Borrowing Current Span
// ============================================================================
//
// Use CurrentSpanLogger when:
// - Helper functions need to log under the current operation
// - You DON'T want to create a new span
// - You want to add logs to an existing span without taking ownership
//
// Key Points:
// - Returns SpanLoggerView (borrowed reference)
// - CANNOT call End() - compile-time safety!
// - No new span created - logs go to current span
// - Perfect for utility functions and helpers

func demonstrateCurrentSpanLogger(ctx context.Context) {
	// Create an owned span for the main operation
	ctx, mainLogger := instrumentation.CreateSpan(ctx, "main_operation")
	defer mainLogger.End()

	mainLogger.Info("Main operation started")

	// Call helper functions that use CurrentSpanLogger
	// These functions will log under the "main_operation" span
	validateInput(ctx, "test-input")
	sanitizeData(ctx, "raw-data")
	formatOutput(ctx, "processed-result")

	mainLogger.Info("Main operation completed")
}

// Helper function 1: Validate input
// Uses CurrentSpanLogger to log under the current span without creating a new one
func validateInput(ctx context.Context, input string) {
	// Get a view of the current span - NO new span created
	view := instrumentation.CurrentSpanLogger(ctx)
	// view.End()  // COMPILE ERROR - method doesn't exist!

	view.Info("Validating input", "input", input)
	view.Debug("Input validation rules applied", "rules_count", 3)
	view.Info("Input validation passed")
}

// Helper function 2: Sanitize data
// Also uses CurrentSpanLogger - all logs go to the same parent span
func sanitizeData(ctx context.Context, data string) {
	view := instrumentation.CurrentSpanLogger(ctx)

	view.Info("Sanitizing data", "data_length", len(data))
	view.Debug("Applying sanitization filters", "filters", "XSS, SQL injection")
	view.Info("Data sanitization complete")
}

// Helper function 3: Format output
// Demonstrates that multiple helpers can safely log to the same span
func formatOutput(ctx context.Context, result string) {
	view := instrumentation.CurrentSpanLogger(ctx)

	view.Info("Formatting output", "format", "JSON")
	view.Debug("Adding response headers", "content_type", "application/json")
	view.Info("Output formatting complete", "size_bytes", len(result)*2)
}

// ============================================================================
// Section 3: CreateRootSpan() - Breaking Parent Chain
// ============================================================================
//
// Use CreateRootSpan when:
// - Starting a new independent trace (new trace ID)
// - Background jobs that shouldn't be part of original request trace
// - Async operations that should be tracked separately
// - Message queue consumers starting fresh work
//
// Key Points:
// - Returns SpanLogger that you OWN
// - Creates NEW trace ID (breaks parent chain)
// - Independent from any existing span in context
// - Still must call defer logger.End()

func demonstrateCreateRootSpan(ctx context.Context) {
	// First, create a parent span to show the contrast
	ctx, parentLogger := instrumentation.CreateSpan(ctx, "original_request_trace")
	defer parentLogger.End()

	parentLogger.Info("Original request started", "request_id", "req-123")

	// Spawn a background job that should NOT be part of this request's trace
	go backgroundJob(ctx)

	// Spawn another independent operation
	go scheduledTask(ctx)

	parentLogger.Info("Original request completed - background jobs are independent")
}

// Background job that creates its own independent trace
func backgroundJob(ctx context.Context) {
	// CreateRootSpan breaks the parent chain - NEW trace ID!
	ctx, logger := instrumentation.CreateRootSpan(ctx, "background_job", "job_id", "job-456")
	defer logger.End()

	logger.Info("Background job started with independent trace")
	logger.Info("This span has its own trace ID - not related to original request")

	// Child spans of this root span will share THIS trace ID
	performBackgroundWork(ctx)

	logger.Info("Background job completed")
}

func performBackgroundWork(ctx context.Context) {
	// This becomes a child of the background_job root span
	ctx, logger := instrumentation.CreateSpan(ctx, "background_work_step")
	defer logger.End()

	logger.Info("Performing background work step")
	logger.Debug("Work details", "items_processed", 42)
}

// Scheduled task that also creates its own trace
func scheduledTask(ctx context.Context) {
	ctx, logger := instrumentation.CreateRootSpan(ctx, "scheduled_task", "task_type", "cleanup")
	defer logger.End()

	logger.Info("Scheduled task started with independent trace")
	logger.Info("This is a separate trace from both the request and background job")

	// Nested operations
	cleanupOldData(ctx)

	logger.Info("Scheduled task completed")
}

func cleanupOldData(ctx context.Context) {
	ctx, logger := instrumentation.CreateSpan(ctx, "cleanup_old_data")
	defer logger.End()

	logger.Info("Cleaning up old data", "days_old", 30)
	logger.Info("Cleanup completed", "rows_deleted", 150)
}
