package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/weka/go-weka-observability/instrumentation"
)

// HTTPServer demonstrates an HTTP server that receives trace context from clients
type HTTPServer struct {
	server *http.Server
	port   string
}

// NewHTTPServer creates a new HTTP server with automatic tracing middleware
func NewHTTPServer(port string) *HTTPServer {
	mux := http.NewServeMux()

	server := &HTTPServer{
		port: port,
	}

	// Add routes - otelhttp will automatically handle trace extraction
	mux.HandleFunc("/api/data", server.handleAPIData)
	mux.HandleFunc("/api/process", server.handleAPIProcess)
	mux.HandleFunc("/health", server.handleHealth)

	// Wrap the entire mux with otelhttp for automatic instrumentation
	handler := otelhttp.NewHandler(mux, "http-server")

	server.server = &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return server
}

// Start starts the HTTP server
func (s *HTTPServer) Start() error {
	fmt.Printf("Starting HTTP server on port %s\n", s.port)

	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *HTTPServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleAPIData demonstrates processing a request with database simulation
func (s *HTTPServer) handleAPIData(w http.ResponseWriter, r *http.Request) {
	// otelhttp automatically extracts trace context, so we can use r.Context() directly
	ctx := r.Context()

	// Create a span for database operation simulation
	ctx, logger := instrumentation.CreateLogSpan(ctx, "database.query")
	defer logger.End()

	logger.Info("Querying database for user data", "query", "SELECT * FROM users")

	// Simulate database query processing time
	time.Sleep(100 * time.Millisecond)

	// Simulate database processing
	s.simulateDataProcessing(ctx)

	// Get current span context for response
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	logger.Info("Database query completed", "rows_returned", 5)

	response := Response{
		Message: "Data retrieved successfully",
		TraceID: traceID,
		SpanID:  spanID,
		Data:    "user_data_123",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleAPIProcess demonstrates a more complex processing workflow
func (s *HTTPServer) handleAPIProcess(w http.ResponseWriter, r *http.Request) {
	// otelhttp automatically extracts trace context, so we can use r.Context() directly
	ctx := r.Context()

	// Create spans for different processing steps
	ctx, logger := instrumentation.CreateLogSpan(ctx, "data.validation")
	logger.Info("Validating input data")
	time.Sleep(50 * time.Millisecond)
	logger.End()

	// Processing step
	ctx, logger = instrumentation.CreateLogSpan(ctx, "data.processing")
	defer logger.End()
	logger.Info("Processing business logic", "step", "transformation")

	// Call external service (simulated)
	s.simulateExternalServiceCall(ctx)

	time.Sleep(150 * time.Millisecond)
	// logger.End() called via defer

	// Final response preparation
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	response := Response{
		Message: "Processing completed successfully",
		TraceID: traceID,
		SpanID:  spanID,
		Data:    "processed_result_456",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleHealth provides a simple health check endpoint
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	// otelhttp automatically extracts trace context, so we can use r.Context() directly
	ctx := r.Context()

	// EXAMPLE: CurrentSpanLogger - log under current span without creating new one
	// Use this pattern when you don't need a new span, just want to log
	view := instrumentation.CurrentSpanLogger(ctx)
	view.Info("Health check requested - logged under current span")

	// For more complex health checks, create a dedicated span
	_, logger := instrumentation.CreateLogSpan(ctx, "health.check")
	defer logger.End()

	logger.Info("Performing detailed health check")

	span := trace.SpanFromContext(ctx)
	response := Response{
		Message: "Server is healthy",
		TraceID: span.SpanContext().TraceID().String(),
		SpanID:  span.SpanContext().SpanID().String(),
		Data:    "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// simulateDataProcessing demonstrates nested span creation for complex operations
func (s *HTTPServer) simulateDataProcessing(ctx context.Context) {
	_, logger := instrumentation.CreateLogSpan(ctx, "data.transform")
	defer logger.End()

	logger.Info("Transforming data", "transformation", "json_to_struct")
	time.Sleep(30 * time.Millisecond)

	// Nested operation
	_, logger2 := instrumentation.CreateLogSpan(ctx, "data.validate")
	defer logger2.End()

	logger2.Info("Validating transformed data", "validation_rules", 3)
	time.Sleep(20 * time.Millisecond)
}

// simulateExternalServiceCall demonstrates how to propagate traces to external HTTP calls
func (s *HTTPServer) simulateExternalServiceCall(ctx context.Context) {
	_, logger := instrumentation.CreateLogSpan(ctx, "external.service_call")
	defer logger.End()

	logger.Info("Calling external service", "service", "analytics-api")

	// This would be where you'd make an actual HTTP call with trace propagation
	// For demonstration, we'll just simulate the processing
	time.Sleep(75 * time.Millisecond)

	logger.Info("External service call completed", "status", "success")
}
