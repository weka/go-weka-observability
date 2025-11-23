//go:build ignore
// +build ignore

package main

import (
	"context"
	"errors"
	"fmt"
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
	).WithName("ErrorPatternsExample")

	ctx = logger.ContextWithLogr(ctx, logr)

	// Setup OpenTelemetry SDK
	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
		ctx,
		"error-patterns-example",
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

	logr.Info("=== Error Handling Patterns: Error() vs SetError() ===")

	// Pattern 1: Error() - Recoverable errors (span stays OK)
	demonstrateRecoverableErrors(ctx)

	// Pattern 2: SetError() - Critical errors (span marked as failed)
	demonstrateCriticalErrors(ctx)

	// Pattern 3: Mixed error handling in complex workflows
	demonstrateComplexErrorHandling(ctx)
}

// ============================================================================
// Pattern 1: Error() - Recoverable Errors
// ============================================================================
//
// Use Error() when:
// - The error is recoverable (operation can continue or retry)
// - The error doesn't represent a failure of the overall operation
// - You want to log the error but the span should remain "OK"
// - Examples: cache miss, optional feature unavailable, first retry attempt
//
// Key Points:
// - Logs the error and records it as a span event
// - Does NOT set span status to Error
// - Span will appear as successful in tracing UIs
// - Use for informational error logging

func demonstrateRecoverableErrors(ctx context.Context) {
	ctx, logger := instrumentation.CreateSpan(ctx, "process_with_recoverable_errors")
	defer logger.End()

	logger.Info("Starting operation that may encounter recoverable errors")

	// Scenario 1: Cache miss - not a failure, just slower path
	err := checkCache(ctx, "user-123")
	if err != nil {
		// Use Error() - cache miss is expected, operation continues
		logger.Error(err, "Cache miss - will fetch from database", "cache_key", "user-123")
	}

	// Scenario 2: Optional feature unavailable - main operation still succeeds
	err = enableOptionalFeature(ctx, "premium-analytics")
	if err != nil {
		// Use Error() - feature unavailable doesn't fail the overall operation
		logger.Error(err, "Optional feature not available", "feature", "premium-analytics")
	}

	// Scenario 3: First retry attempt - expected part of normal flow
	err = callExternalAPI(ctx, "https://api.example.com/data", 1)
	if err != nil {
		// Use Error() - first failure triggers retry, not a span failure
		logger.Error(err, "API call failed, will retry", "attempt", 1)

		// Retry succeeds
		err = callExternalAPI(ctx, "https://api.example.com/data", 2)
		if err == nil {
			logger.Info("API call succeeded on retry", "attempt", 2)
		}
	}

	logger.Info("Operation completed successfully despite recoverable errors")
	// Span status: OK (all errors were recoverable)
}

func checkCache(ctx context.Context, key string) error {
	_, logger := instrumentation.CreateSpan(ctx, "cache_lookup")
	defer logger.End()

	logger.Debug("Looking up cache key", "key", key)

	// Simulate cache miss
	err := errors.New("cache miss")
	logger.Error(err, "Cache key not found - will use fallback", "key", key)

	return err
}

func enableOptionalFeature(ctx context.Context, feature string) error {
	_, logger := instrumentation.CreateSpan(ctx, "enable_optional_feature")
	defer logger.End()

	logger.Debug("Attempting to enable optional feature", "feature", feature)

	// Simulate feature unavailable
	err := fmt.Errorf("feature %s requires premium subscription", feature)
	logger.Error(err, "Optional feature not available - continuing without it", "feature", feature)

	return err
}

func callExternalAPI(ctx context.Context, url string, attempt int) error {
	_, logger := instrumentation.CreateSpan(ctx, "call_external_api")
	defer logger.End()

	logger.Debug("Calling external API", "url", url, "attempt", attempt)

	// First attempt fails, second succeeds
	if attempt == 1 {
		err := errors.New("connection timeout")
		logger.Error(err, "API call failed - transient error", "url", url, "attempt", attempt)
		return err
	}

	logger.Info("API call succeeded", "url", url, "attempt", attempt)
	return nil
}

// ============================================================================
// Pattern 2: SetError() - Critical Errors
// ============================================================================
//
// Use SetError() when:
// - The error represents a failure of the operation
// - The operation cannot complete successfully
// - You want the span to be marked as failed in tracing UIs
// - Examples: validation failure, database error, authentication failure
//
// Key Points:
// - Logs the error and records it as a span event
// - Sets span status to Error (visible in tracing UIs)
// - Span will appear as FAILED with red indicator
// - Use for actual operation failures

func demonstrateCriticalErrors(ctx context.Context) {
	ctx, logger := instrumentation.CreateSpan(ctx, "process_with_critical_errors")
	defer logger.End()

	logger.Info("Starting operation that may encounter critical errors")

	// Scenario 1: Validation failure - operation cannot proceed
	err := validateUserInput(ctx, "")
	if err != nil {
		// Use SetError() - validation failure means operation failed
		logger.SetError(err, "User input validation failed", "reason", "empty_input")
		return // Operation cannot continue
	}

	// This code won't be reached due to validation failure
	logger.Info("Operation completed successfully")
}

func validateUserInput(ctx context.Context, input string) error {
	ctx, logger := instrumentation.CreateSpan(ctx, "validate_user_input")
	defer logger.End()

	logger.Debug("Validating user input", "input_length", len(input))

	if input == "" {
		err := errors.New("input cannot be empty")
		// SetError() marks the span as failed
		logger.SetError(err, "Validation failed - empty input not allowed")
		return err
	}

	logger.Info("User input validation passed")
	return nil
}

// ============================================================================
// Pattern 3: Complex Error Handling
// ============================================================================
//
// Real-world example combining both patterns:
// - Multiple operations with different error severities
// - Some errors are recoverable, others are critical
// - Demonstrates when to use Error() vs SetError()

func demonstrateComplexErrorHandling(ctx context.Context) {
	ctx, logger := instrumentation.CreateSpan(ctx, "complex_workflow")
	defer logger.End()

	logger.Info("Starting complex workflow with mixed error handling")

	// Step 1: Load user data (critical - must succeed)
	user, err := loadUserData(ctx, "user-456")
	if err != nil {
		// Critical error - operation cannot continue
		logger.SetError(err, "Failed to load user data", "user_id", "user-456")
		return
	}
	logger.Info("User data loaded successfully", "user", user)

	// Step 2: Load user preferences (optional - can use defaults)
	preferences, err := loadUserPreferences(ctx, "user-456")
	if err != nil {
		// Recoverable error - use defaults
		logger.Error(err, "Failed to load preferences, using defaults", "user_id", "user-456")
		preferences = getDefaultPreferences()
	}
	logger.Info("User preferences ready", "preferences", preferences)

	// Step 3: Send notification (optional - failure is acceptable)
	err = sendNotification(ctx, "user-456", "Welcome back!")
	if err != nil {
		// Recoverable error - notification failure doesn't fail the operation
		logger.Error(err, "Failed to send notification - user experience unaffected", "user_id", "user-456")
	}

	// Step 4: Update activity log (critical - must succeed for audit trail)
	err = updateActivityLog(ctx, "user-456", "login")
	if err != nil {
		// Critical error - audit trail is mandatory
		logger.SetError(err, "Failed to update activity log - audit requirement violated", "user_id", "user-456")
		return
	}

	logger.Info("Complex workflow completed successfully")
}

func loadUserData(ctx context.Context, userID string) (string, error) {
	ctx, logger := instrumentation.CreateSpan(ctx, "load_user_data")
	defer logger.End()

	logger.Debug("Loading user data from database", "user_id", userID)

	// Simulate successful load
	logger.Info("User data loaded from database", "user_id", userID)
	return "UserData{name: John, email: john@example.com}", nil
}

func loadUserPreferences(ctx context.Context, userID string) (string, error) {
	ctx, logger := instrumentation.CreateSpan(ctx, "load_user_preferences")
	defer logger.End()

	logger.Debug("Loading user preferences", "user_id", userID)

	// Simulate preferences not found (recoverable)
	err := errors.New("preferences not found in cache or database")
	logger.Error(err, "Preferences unavailable - will use defaults", "user_id", userID)
	return "", err
}

func getDefaultPreferences() string {
	return "DefaultPreferences{theme: light, language: en}"
}

func sendNotification(ctx context.Context, userID string, message string) error {
	ctx, logger := instrumentation.CreateSpan(ctx, "send_notification")
	defer logger.End()

	logger.Debug("Sending notification", "user_id", userID, "message", message)

	// Simulate notification service unavailable (recoverable)
	err := errors.New("notification service temporarily unavailable")
	logger.Error(err, "Notification not sent - service unavailable", "user_id", userID)
	return err
}

func updateActivityLog(ctx context.Context, userID string, action string) error {
	ctx, logger := instrumentation.CreateSpan(ctx, "update_activity_log")
	defer logger.End()

	logger.Debug("Updating activity log", "user_id", userID, "action", action)

	// Simulate successful log update
	logger.Info("Activity log updated", "user_id", userID, "action", action)
	return nil
}
