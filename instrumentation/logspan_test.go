package instrumentation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/suite"
	"github.com/weka/go-weka-observability/instrumentation"
	"github.com/weka/go-weka-observability/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// SpanLoggerAPISuite tests the new SpanLogger API with type-safe span ownership
type SpanLoggerAPISuite struct {
	suite.Suite
	ctx       context.Context
	recorder  *tracetest.SpanRecorder
	tp        *trace.TracerProvider
	logOutput *bytes.Buffer // Captures log output for verification
}

func TestSpanLoggerAPISuite(t *testing.T) {
	suite.Run(t, new(SpanLoggerAPISuite))
}

func (s *SpanLoggerAPISuite) SetupTest() {
	// Initialize context first
	s.ctx = context.Background()

	// Create logger with captured output for verification
	s.logOutput = &bytes.Buffer{}
	logr := funcr.NewJSON(func(obj string) {
		s.logOutput.WriteString(obj + "\n")
	}, funcr.Options{
		Verbosity: instrumentation.VerbosityLevelDebug, // Enable debug logs (V(1) level)
	})
	s.ctx = logger.ContextWithLogr(s.ctx, logr)

	// Setup SpanRecorder for simpler span testing
	s.recorder = tracetest.NewSpanRecorder()
	s.tp = trace.NewTracerProvider(
		trace.WithSpanProcessor(s.recorder),
	)

	// Set the global tracer for instrumentation package
	instrumentation.Tracer = s.tp.Tracer("test")
}

func (s *SpanLoggerAPISuite) TearDownTest() {
	if s.tp != nil {
		_ = s.tp.Shutdown(context.Background())
	}
}

// assertLogContains verifies that the log output contains the expected substring
func (s *SpanLoggerAPISuite) assertLogContains(substring string, msgAndArgs ...interface{}) {
	s.Contains(s.logOutput.String(), substring, msgAndArgs...)
}

// resetLogOutput clears the captured log output
func (s *SpanLoggerAPISuite) resetLogOutput() {
	s.logOutput.Reset()
}

// TestCreateSpan_CreatesOwnedSpan verifies CreateSpan returns SpanLogger with End() method
func (s *SpanLoggerAPISuite) TestCreateSpan_CreatesOwnedSpan() {
	ctx, spanLogger := instrumentation.CreateSpan(s.ctx, "test_operation", "key1", "value1")

	s.NotNil(ctx)
	s.NotNil(spanLogger)

	// SpanLogger should have End() method (compile-time check)
	spanLogger.End()

	// Verify span was created and ended
	spans := s.recorder.Ended()
	s.Len(spans, 1)
	s.Equal("test_operation", spans[0].Name())
	s.NotEmpty(spans[0].Attributes())
}

// TestCreateSpan_WithoutKeysAndValues verifies CreateSpan works without optional params
func (s *SpanLoggerAPISuite) TestCreateSpan_WithoutKeysAndValues() {
	ctx, spanLogger := instrumentation.CreateSpan(s.ctx, "simple_operation")

	s.NotNil(ctx)
	s.NotNil(spanLogger)

	spanLogger.End()

	spans := s.recorder.Ended()
	s.Len(spans, 1)
	s.Equal("simple_operation", spans[0].Name())
}

// TestCreateSpan_CreatesChildSpan verifies child spans are properly nested
func (s *SpanLoggerAPISuite) TestCreateSpan_CreatesChildSpan() {
	// Create parent span
	ctx, parentLogger := instrumentation.CreateSpan(s.ctx, "parent_operation")

	// Create child span
	_, childLogger := instrumentation.CreateSpan(ctx, "child_operation")
	childLogger.End()

	// End parent span before checking
	parentLogger.End()

	// Verify parent-child relationship
	spans := s.recorder.Ended()
	s.Len(spans, 2)

	childSpan := spans[0]
	parentSpan := spans[1]

	s.Equal("child_operation", childSpan.Name())
	s.Equal("parent_operation", parentSpan.Name())
	s.Equal(parentSpan.SpanContext().SpanID(), childSpan.Parent().SpanID())
}

// TestCreateRootSpan_BreaksParentChain verifies root spans are independent
func (s *SpanLoggerAPISuite) TestCreateRootSpan_BreaksParentChain() {
	// Create parent span
	ctx, parentLogger := instrumentation.CreateSpan(s.ctx, "parent_operation")

	// Create root span (should NOT be child of parent)
	_, rootLogger := instrumentation.CreateRootSpan(ctx, "root_operation")
	rootLogger.End()

	// End parent span before checking
	parentLogger.End()

	// Verify no parent-child relationship
	spans := s.recorder.Ended()
	s.Len(spans, 2)

	rootSpan := spans[0]
	parentSpan := spans[1]

	s.Equal("root_operation", rootSpan.Name())
	s.Equal("parent_operation", parentSpan.Name())
	s.NotEqual(parentSpan.SpanContext().SpanID(), rootSpan.Parent().SpanID())
	s.False(rootSpan.Parent().IsValid())
}

// TestCurrentSpanLogger_ReturnsBorrowedSpan verifies CurrentSpanLogger returns view
func (s *SpanLoggerAPISuite) TestCurrentSpanLogger_ReturnsBorrowedSpan() {
	// Create a span first
	ctx, spanLogger := instrumentation.CreateSpan(s.ctx, "operation")
	defer spanLogger.End()

	// Get view of current span
	view := instrumentation.CurrentSpanLogger(ctx)

	s.NotNil(view)

	// View should allow logging
	view.Info("test message from view")

	// View should NOT have End() method (compile-time check enforced by type)
	// This is a compile-time check - the following line would not compile:
	// view.End()  // Compile error: view.End undefined
}

// TestSpanLoggerWithValues_ReturnsUpdatedContext verifies WithValues enrichment
func (s *SpanLoggerAPISuite) TestSpanLoggerWithValues_ReturnsUpdatedContext() {
	_, spanLogger := instrumentation.CreateSpan(s.ctx, "operation")

	// WithValues should return updated context and logger
	ctx, enrichedLogger := spanLogger.WithValues("user_id", 123, "tenant", "acme")

	s.NotNil(ctx)
	s.NotNil(enrichedLogger)

	enrichedLogger.Info("enriched log")
	enrichedLogger.End()

	spans := s.recorder.Ended()
	s.Len(spans, 1)

	// Verify enriched attributes were added
	attrs := spans[0].Attributes()
	s.NotEmpty(attrs)

	foundUserId := false
	foundTenant := false
	for _, attr := range attrs {
		if string(attr.Key) == "user_id" && attr.Value.AsInt64() == 123 {
			foundUserId = true
		}
		if string(attr.Key) == "tenant" && attr.Value.AsString() == "acme" {
			foundTenant = true
		}
	}
	s.True(foundUserId, "Expected 'user_id=123' attribute from WithValues")
	s.True(foundTenant, "Expected 'tenant=acme' attribute from WithValues")

	// Verify enriched values appear in logged output
	s.assertLogContains("enriched log", "Logger should have logged the message")
	s.assertLogContains("user_id", "Logger should include user_id in output")
	s.assertLogContains("tenant", "Logger should include tenant in output")
}

// TestSpanLoggerViewWithValues_ReturnsUpdatedContext verifies view WithValues
func (s *SpanLoggerAPISuite) TestSpanLoggerViewWithValues_ReturnsUpdatedContext() {
	// Create a span first
	ctx, spanLogger := instrumentation.CreateSpan(s.ctx, "operation")

	// Get view and enrich it
	view := instrumentation.CurrentSpanLogger(ctx)
	ctx, enrichedView := view.WithValues("request_id", "req-456")

	s.NotNil(ctx)
	s.NotNil(enrichedView)

	enrichedView.Info("enriched view log")

	spanLogger.End()

	// Verify enriched attribute was added by view
	spans := s.recorder.Ended()
	s.Len(spans, 1)

	attrs := spans[0].Attributes()
	foundRequestId := false
	for _, attr := range attrs {
		if string(attr.Key) == "request_id" && attr.Value.AsString() == "req-456" {
			foundRequestId = true
		}
	}
	s.True(foundRequestId, "Expected 'request_id=req-456' attribute from view's WithValues")

	// Verify enriched value appears in logged output
	s.assertLogContains("enriched view log", "Logger should have logged the message")
	s.assertLogContains("request_id", "Logger should include request_id in output")

	// Enriched view still should NOT have End() method
	// This is a compile-time check - would not compile:
	// enrichedView.End()
}

// TestSpanLoggerLoggingMethods verifies all logging methods work
func (s *SpanLoggerAPISuite) TestSpanLoggerLoggingMethods() {
	_, spanLogger := instrumentation.CreateSpan(s.ctx, "operation")

	// Test all logging methods
	spanLogger.Info("info message", "key", "value")
	spanLogger.Debug("debug message", "key", "value")
	spanLogger.Warn("warn message", "key", "value")

	spanLogger.End()

	// Verify span events were created
	spans := s.recorder.Ended()
	s.Len(spans, 1)

	events := spans[0].Events()
	s.Len(events, 3, "Expected 3 events: info, debug, warn")

	// Verify event names match log messages
	s.Equal("info message", events[0].Name)
	s.Equal("debug message", events[1].Name)
	s.Equal("warn message", events[2].Name)

	// Verify attributes were added to span
	attrs := spans[0].Attributes()
	hasKeyValue := false
	hasLevelWarn := false
	for _, attr := range attrs {
		if string(attr.Key) == "key" && attr.Value.AsString() == "value" {
			hasKeyValue = true
		}
		if string(attr.Key) == "level" && attr.Value.AsString() == "warn" {
			hasLevelWarn = true
		}
	}
	s.True(hasKeyValue, "Expected 'key=value' attribute from Info/Debug calls")
	s.True(hasLevelWarn, "Expected 'level=warn' attribute from Warn call")

	// Verify logger was actually called (not just span events)
	s.assertLogContains("info message", "Logger should have logged info message")
	s.assertLogContains("debug message", "Logger should have logged debug message")
	s.assertLogContains("warn message", "Logger should have logged warn message")
}

// TestSpanLoggerErrorMethods verifies error logging behavior
func (s *SpanLoggerAPISuite) TestSpanLoggerErrorMethods() {
	testErr := errors.New("test error")

	// Test Error (logs but doesn't set span status)
	_, spanLogger := instrumentation.CreateSpan(s.ctx, "operation_error")
	spanLogger.Error(testErr, "error occurred", "key", "value")
	spanLogger.End()

	spans := s.recorder.Ended()
	s.Len(spans, 1)
	s.NotEqual(codes.Error, spans[0].Status().Code, "Error() should NOT set span status")

	// Verify error was recorded as an event
	events := spans[0].Events()
	s.NotEmpty(events, "Error() should record error as span event")
	hasErrorEvent := false
	for _, event := range events {
		if event.Name == "exception" {
			hasErrorEvent = true
			break
		}
	}
	s.True(hasErrorEvent, "Expected 'exception' event from Error()")

	// Verify error was logged
	s.assertLogContains("error occurred", "Logger should have logged error message")
	s.assertLogContains("test error", "Logger should have logged error details")

	// Reset log output for next test
	s.resetLogOutput()

	// Test SetError (logs and sets span status to error)
	_, spanLogger2 := instrumentation.CreateSpan(s.ctx, "operation_set_error")
	spanLogger2.SetError(testErr, "error occurred", "key", "value")
	spanLogger2.End()

	spans = s.recorder.Ended()
	s.Len(spans, 2)
	s.Equal(codes.Error, spans[1].Status().Code, "SetError() should set span status to Error")

	// Verify error was recorded as an event
	events2 := spans[1].Events()
	s.NotEmpty(events2, "SetError() should record error as span event")
	hasErrorEvent2 := false
	for _, event := range events2 {
		if event.Name == "exception" {
			hasErrorEvent2 = true
			break
		}
	}
	s.True(hasErrorEvent2, "Expected 'exception' event from SetError()")

	// Verify error was logged by SetError
	s.assertLogContains("error occurred", "Logger should have logged error message from SetError")
	s.assertLogContains("test error", "Logger should have logged error details from SetError")
}

// TestSpanLoggerViewLoggingMethods verifies view logging works
func (s *SpanLoggerAPISuite) TestSpanLoggerViewLoggingMethods() {
	// Create parent span
	ctx, spanLogger := instrumentation.CreateSpan(s.ctx, "parent")

	// Get view
	view := instrumentation.CurrentSpanLogger(ctx)

	// All logging methods should work on view
	view.Info("info from view")
	view.Debug("debug from view")
	view.Warn("warn from view")

	spanLogger.End()

	// Verify span events were created by view's logging methods
	spans := s.recorder.Ended()
	s.Len(spans, 1)

	events := spans[0].Events()
	s.Len(events, 3, "Expected 3 events: info, debug, warn from view")

	// Verify event names match log messages
	s.Equal("info from view", events[0].Name)
	s.Equal("debug from view", events[1].Name)
	s.Equal("warn from view", events[2].Name)

	// Verify logger was actually called by view
	s.assertLogContains("info from view", "Logger should have logged info from view")
	s.assertLogContains("debug from view", "Logger should have logged debug from view")
	s.assertLogContains("warn from view", "Logger should have logged warn from view")
}

// TestSpanLoggerSetAttributes verifies attribute setting
func (s *SpanLoggerAPISuite) TestSpanLoggerSetAttributes() {
	_, spanLogger := instrumentation.CreateSpan(s.ctx, "operation")

	spanLogger.SetAttributes(
		attribute.String("string_attr", "value"),
		attribute.Int("int_attr", 42),
	)

	spanLogger.End()

	spans := s.recorder.Ended()
	s.Len(spans, 1)

	// Verify specific attributes were set
	attrs := spans[0].Attributes()
	s.NotEmpty(attrs)

	foundStringAttr := false
	foundIntAttr := false
	for _, attr := range attrs {
		if string(attr.Key) == "string_attr" && attr.Value.AsString() == "value" {
			foundStringAttr = true
		}
		if string(attr.Key) == "int_attr" && attr.Value.AsInt64() == 42 {
			foundIntAttr = true
		}
	}
	s.True(foundStringAttr, "Expected 'string_attr=value' attribute")
	s.True(foundIntAttr, "Expected 'int_attr=42' attribute")
}

// TestSpanLoggerSetValues verifies SetValues adds to both logger and span
func (s *SpanLoggerAPISuite) TestSpanLoggerSetValues() {
	_, spanLogger := instrumentation.CreateSpan(s.ctx, "operation")

	spanLogger.SetValues("key1", "value1", "key2", 42)

	spanLogger.End()

	spans := s.recorder.Ended()
	s.Len(spans, 1)

	// Verify specific values were set as attributes
	attrs := spans[0].Attributes()
	s.NotEmpty(attrs)

	foundKey1 := false
	foundKey2 := false
	for _, attr := range attrs {
		if string(attr.Key) == "key1" && attr.Value.AsString() == "value1" {
			foundKey1 = true
		}
		if string(attr.Key) == "key2" && attr.Value.AsInt64() == 42 {
			foundKey2 = true
		}
	}
	s.True(foundKey1, "Expected 'key1=value1' attribute")
	s.True(foundKey2, "Expected 'key2=42' attribute")
}

// TestCurrentSpanLogger_WithNoActiveSpan verifies behavior with no span in context
func (s *SpanLoggerAPISuite) TestCurrentSpanLogger_WithNoActiveSpan() {
	// Create context without any span
	ctx := context.Background()
	logr := logger.CreateLogger()
	ctx = logger.ContextWithLogr(ctx, logr)

	// Should still return a view (with no-op span)
	view := instrumentation.CurrentSpanLogger(ctx)

	s.NotNil(view)

	// Should not panic when logging
	s.NotPanics(func() {
		view.Info("test message")
	})
}

// ExampleCreateSpan demonstrates creating an owned span with deferred cleanup.
func ExampleCreateSpan() {
	// Setup (in real code, use instrumentation.SetupOTelSDK)
	ctx := context.Background()

	// Create a span - you own it and must call End()
	_, logger := instrumentation.CreateSpan(ctx, "process_request", "user_id", 123)
	defer logger.End() // Required!

	// Log messages automatically create span events
	logger.Info("Processing user request")
	logger.Debug("Detailed processing info")

	// The span is automatically a child of any existing span in ctx
}

// ExampleCurrentSpanLogger demonstrates borrowing the current span.
func ExampleCurrentSpanLogger() {
	// Setup
	ctx := context.Background()
	ctx, parentLogger := instrumentation.CreateSpan(ctx, "parent_operation")
	defer parentLogger.End()

	// In a helper function, get a view of the current span
	helperFunction := func(ctx context.Context) {
		view := instrumentation.CurrentSpanLogger(ctx)

		// Can log under the current span
		view.Info("Helper function working")

		// CANNOT call view.End() - compile error!
		// This prevents accidentally ending a span you don't own
	}

	helperFunction(ctx)
}

// ExampleCreateRootSpan demonstrates starting a new trace.
func ExampleCreateRootSpan() {
	// Setup - imagine this is called from a message queue handler
	ctx := context.Background()

	// Create a root span - starts completely new trace
	_, logger := instrumentation.CreateRootSpan(ctx, "background_job", "job_id", "abc-123")
	defer logger.End()

	// This span has its own trace ID, independent of any parent
	logger.Info("Background job started")
	logger.Info("Job processing completed")
}

// Example_withValues demonstrates enriching logger context.
func Example_withValues() {
	ctx := context.Background()
	_, logger := instrumentation.CreateSpan(ctx, "process_order")
	defer logger.End()

	// Enrich context with additional values
	_, enrichedLogger := logger.WithValues("order_id", "ORD-456", "customer_id", 789)

	// All logs from enrichedLogger include order_id and customer_id
	enrichedLogger.Info("Order validated")
	enrichedLogger.Info("Order processed")
}

// Example_errorHandling demonstrates error logging patterns.
func Example_errorHandling() {
	ctx := context.Background()
	_, logger := instrumentation.CreateSpan(ctx, "operation")
	defer logger.End()

	// Log an error without marking span as failed (recoverable error)
	if err := someRecoverableOperation(); err != nil {
		logger.Error(err, "Recoverable error occurred", "attempt", 1)
		// Span status remains OK
	}

	// Log an error AND mark span as failed (critical error)
	if err := someCriticalOperation(); err != nil {
		logger.SetError(err, "Critical error occurred")
		// Span status = Error (visible in tracing UI)
		return
	}
}

// Helper functions for examples
func someRecoverableOperation() error { return nil }
func someCriticalOperation() error    { return nil }
