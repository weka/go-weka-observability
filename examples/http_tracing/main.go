package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/zerologr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

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

	logger.SetCallerDirDisplayLevel()
}

// HTTPServer demonstrates an HTTP server that receives trace context from clients
type HTTPServer struct {
	server *http.Server
	port   string
}

// Response represents the JSON response from the server
type Response struct {
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
	Data    string `json:"data"`
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
		Addr:    ":" + port,
		Handler: handler,
	}

	return server
}

// handleAPIData demonstrates processing a request with database simulation
func (s *HTTPServer) handleAPIData(w http.ResponseWriter, r *http.Request) {
	// otelhttp automatically extracts trace context, so we can use r.Context() directly
	ctx := r.Context()

	// Create a span for database operation simulation
	ctx, logger, endSpan := instrumentation.GetLogSpan(ctx, "database.query")
	defer endSpan()

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
	ctx, logger, endSpan := instrumentation.GetLogSpan(ctx, "data.validation")
	logger.Info("Validating input data")
	time.Sleep(50 * time.Millisecond)
	endSpan()

	// Processing step
	ctx, logger, endSpan = instrumentation.GetLogSpan(ctx, "data.processing")
	logger.Info("Processing business logic", "step", "transformation")

	// Call external service (simulated)
	s.simulateExternalServiceCall(ctx)

	time.Sleep(150 * time.Millisecond)
	endSpan()

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
	_, logger, endSpan := instrumentation.GetLogSpan(ctx, "health.check")
	defer endSpan()

	logger.Info("Health check requested")

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
	_, logger, endSpan := instrumentation.GetLogSpan(ctx, "data.transform")
	defer endSpan()

	logger.Info("Transforming data", "transformation", "json_to_struct")
	time.Sleep(30 * time.Millisecond)

	// Nested operation
	_, logger2, endSpan2 := instrumentation.GetLogSpan(ctx, "data.validate")
	defer endSpan2()

	logger2.Info("Validating transformed data", "validation_rules", 3)
	time.Sleep(20 * time.Millisecond)
}

// simulateExternalServiceCall demonstrates how to propagate traces to external HTTP calls
func (s *HTTPServer) simulateExternalServiceCall(ctx context.Context) {
	_, logger, endSpan := instrumentation.GetLogSpan(ctx, "external.service_call")
	defer endSpan()

	logger.Info("Calling external service", "service", "analytics-api")

	// This would be where you'd make an actual HTTP call with trace propagation
	// For demonstration, we'll just simulate the processing
	time.Sleep(75 * time.Millisecond)

	logger.Info("External service call completed", "status", "success")
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

// HTTPClient demonstrates an HTTP client that propagates trace context to servers
type HTTPClient struct {
	client  *http.Client
	baseURL string
}

// NewHTTPClient creates a new HTTP client with automatic tracing support
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		baseURL: baseURL,
	}
}

// Get performs a GET request with automatic trace propagation via otelhttp
func (c *HTTPClient) Get(ctx context.Context, endpoint string) (*Response, error) {
	// Create a span for this business logic operation
	ctx, logger, endSpan := instrumentation.GetLogSpan(ctx, fmt.Sprintf("client.get_%s", endpoint))
	defer endSpan()

	url := c.baseURL + endpoint
	logger.Info("Making HTTP GET request", "url", url)

	// Create the HTTP request - otelhttp will automatically handle trace propagation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.Error(err, "Failed to create HTTP request")
		return nil, err
	}

	// Add custom headers
	req.Header.Set("User-Agent", "go-weka-observability-example/1.0")
	req.Header.Set("Accept", "application/json")

	// Make the HTTP request - otelhttp transport handles tracing automatically
	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error(err, "HTTP request failed")
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	logger.SetValues("http.status_code", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
		logger.Error(err, "Unexpected HTTP status code")
		return nil, err
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(err, "Failed to read response body")
		return nil, err
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Error(err, "Failed to parse response JSON")
		return nil, err
	}

	logger.Info("HTTP request completed successfully",
		"response_trace_id", response.TraceID,
		"response_span_id", response.SpanID,
		"message", response.Message)

	return &response, nil
}

// Post performs a POST request with automatic trace propagation via otelhttp
func (c *HTTPClient) Post(ctx context.Context, endpoint string, data any) (*Response, error) {
	// Create a span for this business logic operation
	ctx, logger, endSpan := instrumentation.GetLogSpan(ctx, fmt.Sprintf("client.post_%s", endpoint))
	defer endSpan()

	url := c.baseURL + endpoint
	logger.Info("Making HTTP POST request", "url", url)

	// Serialize request data
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			logger.Error(err, "Failed to marshal request data")
			return nil, err
		}
		body = bytes.NewReader(jsonData)
	}

	// Create the HTTP request - otelhttp will automatically handle trace propagation
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		logger.Error(err, "Failed to create HTTP request")
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-weka-observability-example/1.0")

	// Make the request - otelhttp transport handles tracing automatically
	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error(err, "HTTP request failed")
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	logger.SetValues("http.status_code", resp.StatusCode)

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(err, "Failed to read response body")
		return nil, err
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		logger.Error(err, "Failed to parse response JSON")
		return nil, err
	}

	logger.Info("HTTP POST request completed",
		"response_trace_id", response.TraceID,
		"message", response.Message)

	return &response, nil
}

// main demonstrates end-to-end trace propagation between client and server
func main() {
	ctx := context.Background()

	// Initialize root logger and put it into context
	logr := zerologr.New(logger.NewZeroLogger())
	ctx, ctxLogger := instrumentation.GetLoggerForContext(ctx, &logr, "HTTPTracingExample")

	// Setup OpenTelemetry SDK
	shutdown, err := instrumentation.SetupOTelSDK(ctx, "http-tracing-example", "v1.0.0", ctxLogger)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			ctxLogger.Error(err, "Failed to shutdown OTel SDK")
		}
	}()

	// Create and start HTTP server in goroutine
	server := NewHTTPServer("8080")
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			ctxLogger.Error(err, "HTTP server error")
		}
	}()

	// Give server time to start
	time.Sleep(1 * time.Second)

	// Create HTTP client
	client := NewHTTPClient("http://localhost:8080")

	// Start a root trace for the client operations
	ctx, rootLogger, endRootSpan := instrumentation.GetLogSpan(ctx, "client.workflow")
	defer endRootSpan()

	rootLogger.Info("Starting client workflow demonstration")

	// Demonstrate multiple HTTP calls within the same trace

	// 1. Health check
	healthCheckCtx, healthLogger, endHealthSpan := instrumentation.GetLogSpan(ctx, "workflow.health_check")
	healthResp, err := client.Get(healthCheckCtx, "/health")
	if err != nil {
		healthLogger.Error(err, "Health check failed")
	} else {
		healthLogger.Info("Health check successful", "server_trace_id", healthResp.TraceID)
	}
	endHealthSpan()

	// 2. Data retrieval
	getDataCtx, dataLogger, endDataSpan := instrumentation.GetLogSpan(ctx, "workflow.get_data")
	dataResp, err := client.Get(getDataCtx, "/api/data")
	if err != nil {
		dataLogger.Error(err, "Data retrieval failed")
	} else {
		dataLogger.Info("Data retrieved successfully",
			"server_trace_id", dataResp.TraceID,
			"data", dataResp.Data)
	}
	endDataSpan()

	// 3. Data processing
	processDataCtx, processLogger, endProcessSpan := instrumentation.GetLogSpan(ctx, "workflow.process_data")
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
	endProcessSpan()

	rootLogger.Info("Client workflow completed successfully")

	// Gracefully shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		ctxLogger.Error(err, "Failed to shutdown server")
	}

	ctxLogger.Info("HTTP trace propagation example completed")
}
