package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/weka/go-weka-observability/instrumentation"
)

// errUnexpectedStatusCode is returned when HTTP response has unexpected status code.
var errUnexpectedStatusCode = errors.New("HTTP request failed with unexpected status code")

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
	ctx, spanLog := instrumentation.CreateLogSpan(ctx, "client.get_"+endpoint)
	defer spanLog.End()

	url := c.baseURL + endpoint
	spanLog.Info("Making HTTP GET request", "url", url)

	// Create the HTTP request - otelhttp will automatically handle trace propagation
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		spanLog.Error(err, "Failed to create HTTP request")

		return nil, err
	}

	// Add custom headers
	req.Header.Set("User-Agent", "go-weka-observability-example/1.0")
	req.Header.Set("Accept", "application/json")

	// Make the HTTP request - otelhttp transport handles tracing automatically
	resp, err := c.client.Do(req)
	if err != nil {
		spanLog.Error(err, "HTTP request failed")

		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	spanLog.SetValues("http.status_code", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		spanLog.Error(errUnexpectedStatusCode, "Unexpected HTTP status code", "status_code", resp.StatusCode)

		return nil, fmt.Errorf("%w: %d", errUnexpectedStatusCode, resp.StatusCode)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		spanLog.Error(err, "Failed to read response body")

		return nil, err
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		spanLog.Error(err, "Failed to parse response JSON")

		return nil, err
	}

	spanLog.Info("HTTP request completed successfully",
		"response_trace_id", response.TraceID,
		"response_span_id", response.SpanID,
		"message", response.Message)

	return &response, nil
}

// Post performs a POST request with automatic trace propagation via otelhttp
func (c *HTTPClient) Post(ctx context.Context, endpoint string, data any) (*Response, error) {
	// Create a span for this business logic operation
	ctx, spanLog := instrumentation.CreateLogSpan(ctx, "client.post_"+endpoint)
	defer spanLog.End()

	url := c.baseURL + endpoint
	spanLog.Info("Making HTTP POST request", "url", url)

	// Serialize request data
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			spanLog.Error(err, "Failed to marshal request data")

			return nil, err
		}
		body = bytes.NewReader(jsonData)
	}

	// Create the HTTP request - otelhttp will automatically handle trace propagation
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		spanLog.Error(err, "Failed to create HTTP request")

		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-weka-observability-example/1.0")

	// Make the request - otelhttp transport handles tracing automatically
	resp, err := c.client.Do(req)
	if err != nil {
		spanLog.Error(err, "HTTP request failed")

		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	spanLog.SetValues("http.status_code", resp.StatusCode)

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		spanLog.Error(err, "Failed to read response body")

		return nil, err
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		spanLog.Error(err, "Failed to parse response JSON")

		return nil, err
	}

	spanLog.Info("HTTP POST request completed",
		"response_trace_id", response.TraceID,
		"message", response.Message)

	return &response, nil
}
