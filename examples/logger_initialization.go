//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"

	zerologger "github.com/weka/go-weka-observability/logger"
)

// Example: Old way (confusing)
func oldWay() {
	// Confusing: passing nil pointer, double logger creation
	// ctx := context.Background()
	// logr := zerologr.New(logger.NewZeroLogger())
	// ctx, ctxLogger := instrumentation.GetLoggerForContext(ctx, &logr, "my-service")

	fmt.Println("❌ Old way is deprecated - see new examples below")
}

// Example: New way - Simple initialization
func newWaySimple() {
	ctx := context.Background()

	// Create logger with environment defaults
	logr := zerologger.CreateLoggerFrom(zerologger.NewDefaultConfigWithEnvOverrides())

	// Store in context
	ctx = zerologger.ContextWithLogr(ctx, logr)

	// Get logger from context
	logger := zerologger.MustLogrFromContext(ctx)
	logger.Info("Operation started")

	fmt.Println("✅ Simple: Logger initialized with environment defaults")
}

// Example: New way - Console logger
func newWayConsole() {
	ctx := context.Background()

	// Create console logger explicitly using functional options
	logr := zerologger.CreateLogger(
		zerologger.WithConsoleSink(),
		zerologger.WithInfoLevel(),
	)

	// Store in context and add service name
	logr = logr.WithName("my-service")
	ctx = zerologger.ContextWithLogr(ctx, logr)

	logr.Info("Service initialized with console output")
	fmt.Println("✅ Console: Logger with explicit console mode")
}

// Example: New way - File logger
func newWayFile() {
	ctx := context.Background()

	// Create file logger using functional options
	logr := zerologger.CreateLogger(
		zerologger.WithFileSink("test-logs", "example.log"),
		zerologger.WithRotation(10, 2, 1),
		zerologger.WithInfoLevel(),
	)

	ctx = zerologger.ContextWithLogr(ctx, logr)

	logger := zerologger.MustLogrFromContext(ctx)
	logger.Info("This goes to test-logs/example.log")
	fmt.Println("✅ File: Logger writing to test-logs/example.log")
}

// Example: New way - Graceful fallback
func newWayGraceful() {
	ctx := context.Background()

	// No logger in context - gracefully handles it
	logger, err := zerologger.LogrFromContext(ctx)
	if err != nil {
		fmt.Println("⚠️  No logger in context, creating default...")
		// Handle gracefully - create one with defaults
		logr := zerologger.CreateLoggerFrom(zerologger.NewDefaultConfigWithEnvOverrides())
		ctx = zerologger.ContextWithLogr(ctx, logr)
		logger = zerologger.MustLogrFromContext(ctx)
	}

	logger.Info("Graceful fallback worked")
	fmt.Println("✅ Graceful: Error handling allows fallback strategy")
}

// Example: New way - Must pattern (panics if missing)
func newWayMust() {
	ctx := context.Background()

	// Create and store logger first
	logr := zerologger.CreateLoggerFrom(zerologger.NewDefaultConfigWithEnvOverrides())
	ctx = zerologger.ContextWithLogr(ctx, logr)

	// This panics if logger not found - use when logger is required
	logger := zerologger.MustLogrFromContext(ctx)
	logger.Info("Must pattern works")
	fmt.Println("✅ Must: Panic-based retrieval for required logger")
}

// Example: Using existing logger (ContextWithLogr)
func existingLoggerExample() {
	ctx := context.Background()

	// You already have a logger configured
	logr := zerologger.CreateLogger(
		zerologger.WithConsoleSink(),
	).WithName("pre-configured")

	// Store it in context using ContextWithLogr
	ctx = zerologger.ContextWithLogr(ctx, logr)

	// Now retrieve and use it
	retrievedLogger := zerologger.MustLogrFromContext(ctx)
	retrievedLogger.Info("Using existing logger from context")

	fmt.Println("✅ Existing logger: Stored pre-configured logger in context")
}

// Example: Real-world initialization in main()
func realWorldExample() {
	ctx := context.Background()

	// 1. Create root logger (respects LOG_MODE, LOG_DIR env vars)
	logr := zerologger.CreateLoggerFrom(zerologger.NewDefaultConfigWithEnvOverrides())
	ctx = zerologger.ContextWithLogr(ctx, logr)

	// 2. Get logger for OTel setup
	ctxLogger := zerologger.MustLogrFromContext(ctx).WithName("otel-setup")

	// 3. Setup OTel (example - would call real SetupOTelSDK)
	serviceName := "my-service"
	version := "1.0.0"
	ctxLogger.Info("Setting up OTel", "service", serviceName, "version", version)

	// 4. Get logger for main operation
	mainLogger := zerologger.MustLogrFromContext(ctx).WithName("main")
	mainLogger.Info("Application started")

	fmt.Println("✅ Real-world: Complete initialization pattern")
}

func main() {
	fmt.Println("=== Logger Initialization Examples ===\n")

	fmt.Println("1. Simple initialization (env-aware):")
	newWaySimple()

	fmt.Println("\n2. Console logger:")
	newWayConsole()

	fmt.Println("\n3. File logger:")
	newWayFile()

	fmt.Println("\n4. Graceful error handling:")
	newWayGraceful()

	fmt.Println("\n5. Must pattern:")
	newWayMust()

	fmt.Println("\n6. Existing logger (ContextWithLogger):")
	existingLoggerExample()

	fmt.Println("\n7. Real-world example:")
	realWorldExample()

	fmt.Println("\n=== Summary ===")
	fmt.Println("✅ logger.CreateLoggerFrom(config) - Create logger with config")
	fmt.Println("✅ logger.ContextWithLogr(ctx, logr) - Store logger in context")
	fmt.Println("✅ logger.LogrFromContext(ctx) - Get logger with error handling")
	fmt.Println("✅ logger.MustLogrFromContext(ctx) - Get logger or panic")
	fmt.Println("✅ logger.LogrFromContextOrDefault(ctx) - Get logger with fallback")
}
