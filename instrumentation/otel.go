// Package instrumentation provides OpenTelemetry tracing integration with smart
// tracer management, automatic provider detection, and combined logging/tracing.
//
// # Key Features
//
//   - Smart tracer resolution with automatic provider change detection
//   - Context-based tracer injection for test isolation
//   - Lazy tracer initialization with thread-safe caching
//   - Automatic OpenTelemetry SDK setup with sensible defaults
//   - OTLP gRPC exporter for trace export
//   - Combined logging and tracing via SpanLogger API
//   - Environment variable overrides (OTEL_EXPORTER_OTLP_ENDPOINT)
//   - Resource attributes for service identification
//   - Graceful degradation when no collector is available
//   - Test helpers for parallel and sequential test isolation
//
// # Quick Start
//
// Basic setup with logger:
//
//	logr := logger.CreateLogger()
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "my-service", "v1.0.0", logr,
//	    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
//	)
//	if err != nil {
//	    return err
//	}
//	defer shutdown(ctx)
//
// Combined logging and tracing:
//
//	// CreateLogSpan creates a span and returns a logger automatically enriched with trace IDs
//	ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "operation-name", "user_id", 123)
//	defer spanLogger.End()
//
//	spanLogger.Info("Processing request")
//	// Logs include trace_id, span_id, and all key-value pairs automatically
//
// # Environment Variables
//
//   - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP collector endpoint (overrides code defaults)
//
// Example: OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
//
// # Configuration Patterns
//
// Functional options (recommended):
//
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "service", "v1.0.0", logr,
//	    instrumentation.WithDefaultOTLPEndpoint("http://collector:4317"),
//	    instrumentation.WithResourceAttributes("environment", "production"),
//	)
//
// Explicit config:
//
//	config := instrumentation.OTelConfig{
//	    Endpoint: "http://collector:4317",
//	    ResourceAttributes: []any{"environment", "production"},
//	}
//	config = instrumentation.NewOTelConfigFromEnv(config)
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "service", "v1.0.0", logr, config)
//
// # Complete Example
//
//	func main() {
//	    ctx := context.Background()
//
//	    // Setup logger
//	    logr := logger.CreateLogger(logger.WithInfoLevel())
//	    ctx = logger.ContextWithLogr(ctx, logr) // IMPORTANT: Store in context!
//
//	    // Setup OpenTelemetry
//	    shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	        ctx, "my-service", "v1.0.0", logr,
//	        instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
//	    )
//	    if err != nil {
//	        panic(err)
//	    }
//	    defer shutdown(ctx)
//
//	    // Use traced logging (logger retrieved from context)
//	    processRequest(ctx)
//	}
//
//	func processRequest(ctx context.Context) {
//	    ctx, logger, end := instrumentation.GetLogSpan(ctx, "process_request")
//	    defer end()
//
//	    logger.Info("Processing started", "request_id", "req-123")
//	    // Span is automatically created and logs include trace IDs
//
//	    // Nested operations create child spans
//	    queryDatabase(ctx)
//	}
//
//	func queryDatabase(ctx context.Context) {
//	    ctx, logger, end := instrumentation.GetLogSpan(ctx, "query_database")
//	    defer end()
//
//	    logger.Info("Querying database", "query", "SELECT * FROM users")
//	    // This span is a child of process_request span
//	}
//
// # Reusing Parent Span (Helper Functions)
//
// Sometimes you want a helper function to log under the parent's span without creating
// a new span. Use an empty string for the span name, and DO NOT call end():
//
//	func processRequest(ctx context.Context) {
//	    ctx, logger, end := instrumentation.GetLogSpan(ctx, "process_request")
//	    defer end()
//
//	    logger.Info("Processing started")
//	    helper(ctx) // Helper logs under same span
//	}
//
//	func helper(ctx context.Context) {
//	    _, logger, _ := instrumentation.GetLogSpan(ctx, "")
//	    // Calling end() is safe (no-op) but not calling it makes it clearer
//	    // that the parent owns the span lifecycle
//	    logger.Info("Helper doing work") // Uses parent's span
//	}
//
// # Trace Management System
//
// The package uses smart tracer resolution with automatic provider detection
// and context-based injection for test isolation.
//
// Tracer Resolution Strategy (Three-Tier):
//
//  1. Context Override - ContextWithTracer(ctx, tracer) takes priority
//  2. Cached Tracer - Fast path with automatic provider change detection
//  3. Provider Creation - Lazy initialization from otel.GetTracerProvider()
//
// Performance Characteristics:
//   - Production (provider never changes): ~45-75ns per CreateSpan
//   - Tests (provider swap detected): ~1-10μs on first span after swap
//   - Thread-safe caching with double-check locking pattern
//
// Testing Patterns:
//
// Parallel tests (context-based isolation):
//
//	func TestFeature(t *testing.T) {
//	    t.Parallel()  // ✅ Safe!
//
//	    ctx := context.Background()
//	    ctx, recorder := instrumentation.SetupOTELTester(ctx)
//	    defer recorder.Shutdown(context.Background())
//
//	    ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
//	    defer logger.End()
//
//	    // Verify spans
//	    spans := recorder.Ended()
//	}
//
// Sequential tests (provider-based, simpler):
//
//	func TestFeature(t *testing.T) {
//	    // ⚠️ NO t.Parallel() - swaps global provider
//
//	    ctx := context.Background()
//	    ctx, recorder := instrumentation.SetupOTELTesterWithProvider(ctx)
//	    defer recorder.Shutdown(context.Background())
//
//	    ctx, logger := instrumentation.CreateLogSpan(ctx, "operation")
//	    defer logger.End()
//	}
//
// For complete documentation see docs/trace-management.md.
//
// See package documentation and examples for more details.
package instrumentation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// OTLPExporterTimeout defines the maximum time to wait when establishing
	// a connection to the OTLP endpoint during trace exporter initialization
	OTLPExporterTimeout = 5 * time.Second

	// OTLPBatchTimeout defines how frequently batched traces are exported.
	// Lower values reduce latency but increase network overhead and batching efficiency.
	// OpenTelemetry default is 5s; we use 1s for faster trace visibility.
	OTLPBatchTimeout = time.Second
)

// SetupOTelSDKFrom bootstraps the OpenTelemetry pipeline with explicit configuration.
// If it does not return an error, make sure to call shutdown for proper cleanup.
//
// This function follows the same pattern as logger.CreateLoggerFrom - you provide a config
// that can include defaults overridden by environment variables.
//
// Example with environment defaults:
//
//	logr := logger.CreateLogger()
//	config := instrumentation.NewDefaultOTelConfigWithEnvOverrides()
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "my-service", "v1.0.0", logr, config, "key", "value")
//	if err != nil {
//	    return err
//	}
//	defer shutdown(ctx)
//
// Example with custom defaults that can be overridden by env:
//
//	logr := logger.CreateLogger()
//	config := instrumentation.OTelConfig{
//	    Endpoint: "http://default-collector:4317",  // This is the DEFAULT
//	}
//	config = instrumentation.NewOTelConfigFromEnv(config)  // Env can override
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "my-service", "v1.0.0", logr, config, "key", "value")
func SetupOTelSDKFrom(
	ctx context.Context,
	serviceName, serviceVersion string,
	logger logr.Logger,
	config OTelConfig,
	keysAndValues ...any,
) (shutdown func(context.Context) error, err error) {
	logger.
		V(VerbosityLevelDebug).
		WithCallDepth(CallDepthOffset).
		Info("Setting up OTel SDK", "service", serviceName, "version", serviceVersion)

	// Merge provided keysAndValues with config's ResourceAttributes
	// Copy ResourceAttributes to avoid mutating the caller's config
	if len(keysAndValues) > 0 {
		merged := make([]any, 0, len(config.ResourceAttributes)+len(keysAndValues))
		merged = append(merged, config.ResourceAttributes...)
		merged = append(merged, keysAndValues...)
		config.ResourceAttributes = merged
	}

	return setupOTelSDKInternal(ctx, serviceName, serviceVersion, logger, config)
}

// SetupOTelSDKWithOptions bootstraps the OpenTelemetry pipeline with functional options.
// If it does not return an error, make sure to call shutdown for proper cleanup.
//
// This function follows the same pattern as logger.CreateLogger - functional options
// set defaults, then environment variables (OTEL_EXPORTER_OTLP_ENDPOINT) can override.
//
// # Tracer Provider Setup
//
// This function sets the global TracerProvider via otel.SetTracerProvider(), which becomes
// the source for all tracers in your application. You do NOT need to manually manage tracers
// or store them in context - CreateLogSpan/CreateRootLogSpan automatically resolve the correct tracer
// from the provider.
//
//	SetupOTelSDKWithOptions() → otel.SetTracerProvider(provider)
//	CreateLogSpan(ctx, "op") → GetTracer(ctx) → otel.GetTracerProvider().Tracer(...)
//
// The tracer resolution uses smart caching with provider change detection, so it's both
// performant (cached reads) and test-friendly (detects provider swaps automatically).
//
// # Logger Context Independence
//
// IMPORTANT: The logger parameter is used ONLY for logging during SDK initialization.
// It is NOT automatically stored in context.
//
// You must call logger.ContextWithLogr() BEFORE calling CreateLogSpan, but the order
// between ContextWithLogr() and SetupOTelSDKWithOptions() does NOT matter.
//
//	Recommended pattern (SetupOTelSDK first):
//	  1. CreateLogger() - Creates logger instance
//	  2. SetupOTelSDKWithOptions() - Sets TracerProvider via otel.SetTracerProvider()
//	  3. ContextWithLogr() - Stores logger for CreateLogSpan to retrieve
//	  4. CreateLogSpan() - Retrieves logger from context, tracer from provider
//
//	Alternative pattern (ContextWithLogr first):
//	  1. CreateLogger() - Creates logger instance
//	  2. ContextWithLogr() - Stores logger for CreateLogSpan to retrieve
//	  3. SetupOTelSDKWithOptions() - Sets TracerProvider via otel.SetTracerProvider()
//	  4. CreateLogSpan() - Retrieves logger from context, tracer from provider
//
// # Complete Example
//
//	// Create logger (overrideable via LOG_* env vars)
//	logr := logger.CreateLogger(
//	    logger.WithConsoleSink(),
//	    logger.WithInfoLevel(),
//	)
//
//	// Setup OpenTelemetry (sets global TracerProvider)
//	// OTEL_EXPORTER_OTLP_ENDPOINT env var can override endpoint
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "my-service", "v1.0.0", logr,
//	    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
//	    instrumentation.WithResourceAttributes("environment", "production"),
//	)
//	if err != nil {
//	    return err
//	}
//	defer shutdown(ctx)
//
//	// Store logger in context for CreateLogSpan to use (order doesn't matter vs SetupOTelSDK)
//	ctx = logger.ContextWithLogr(ctx, logr)
//
//	// CreateLogSpan automatically gets tracer from provider - no manual tracer management needed
//	ctx, spanLogger := instrumentation.CreateLogSpan(ctx, "operation", "user_id", 123)
//	defer spanLogger.End()
//	spanLogger.Info("Processing request")
func SetupOTelSDKWithOptions(
	ctx context.Context,
	serviceName, serviceVersion string,
	logger logr.Logger,
	opts ...OTelOption,
) (shutdown func(context.Context) error, err error) {
	logger.
		V(VerbosityLevelDebug).
		WithCallDepth(CallDepthOffset).
		Info("Setting up OTel SDK", "service", serviceName, "version", serviceVersion)

	// Start with defaults
	config := DefaultOTelConfig()

	// Apply functional options to set fallback values
	for _, opt := range opts {
		opt(&config)
	}

	// Environment variables always take precedence if set
	config = NewOTelConfigFromEnv(config)

	return setupOTelSDKInternal(ctx, serviceName, serviceVersion, logger, config)
}

// setupOTelSDKInternal contains the actual OpenTelemetry SDK initialization logic.
// This is extracted to be reused by both SetupOTelSDK and SetupOTelSDKWithOptions.
func setupOTelSDKInternal(
	ctx context.Context,
	serviceName, serviceVersion string,
	logger logr.Logger,
	config OTelConfig,
) (shutdown func(context.Context) error, err error) {
	// Initialize tracer cache (triggers getTracer on first use)
	// The public Tracer variable is kept in sync automatically by GetTracer()
	// for backward compatibility

	if config.Endpoint == "" {
		logger.V(VerbosityLevelInfo).
			WithCallDepth(CallDepthOffset).
			Info("No OTLP endpoint configured - traces will not be exported")

		return func(_ context.Context) error {
			return nil
		}, nil
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTraceProvider(
		ctx,
		serviceName,
		serviceVersion,
		config.Endpoint,
		config.DisableGRPCServiceConfig,
		logger,
		config.ResourceAttributes...)
	if err != nil {
		handleErr(err)

		return shutdown, err
	}
	otel.SetTracerProvider(tracerProvider) // <-- Magic unleashed! 🎉

	return func(ctx context.Context) error {
		err = tracerProvider.ForceFlush(context.Background())
		if err != nil {
			logger.Error(err, "failed to flush traces")
		}

		return tracerProvider.Shutdown(ctx)
	}, err
}

func newResource(
	ctx context.Context,
	serviceName, serviceVersion string,
	keysAndValues ...any,
) (*resource.Resource, error) {
	// Add any additional resource attributes provided using the existing helper function
	additionalAttrs := getAttributesFromKeysAndValues(keysAndValues...)

	// Preallocate with known capacity: 2 base attributes + additional
	baseAttrCount := 2
	attrs := make([]attribute.KeyValue, 0, baseAttrCount+len(additionalAttrs))
	attrs = append(attrs,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(serviceVersion),
	)
	attrs = append(attrs, additionalAttrs...)

	// Create resource without SchemaURL to avoid community-reported conflicts
	customResource, err := resource.New(
		ctx,
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, err
	}

	r, err := resource.Merge(
		resource.Default(),
		customResource,
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(
	ctx context.Context,
	serviceName, serviceVersion, endpoint string,
	disableGRPCServiceConfig bool,
	logger logr.Logger,
	keysAndValues ...any,
) (*tracesdk.TracerProvider, error) {
	logger.Info("Setting up OTel trace provider", "service", serviceName, "version", serviceVersion)
	var traceProvider *tracesdk.TracerProvider

	if endpoint != "" {
		logger.Info("OTLP endpoint set", "endpoint", endpoint)

		securityOption := otlptracegrpc.WithInsecure()
		if strings.Contains(endpoint, "https://") {
			securityOption = otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
		}
		exporterOpts := []otlptracegrpc.Option{
			securityOption,
			otlptracegrpc.WithTimeout(OTLPExporterTimeout),
			otlptracegrpc.WithEndpointURL(endpoint),
		}
		if disableGRPCServiceConfig {
			exporterOpts = append(exporterOpts, otlptracegrpc.WithDialOption(grpc.WithDisableServiceConfig()))
		}
		traceExporter, err := otlptracegrpc.New(ctx, exporterOpts...)
		if err != nil {
			logger.Error(err, "failed to create OTLP trace exporter")

			return nil, err
		}

		res, err := newResource(ctx, serviceName, serviceVersion, keysAndValues...)
		if err != nil {
			logger.Error(err, "failed to create resource")

			return nil, err
		}

		traceProvider = tracesdk.NewTracerProvider(
			tracesdk.WithBatcher(traceExporter,
				tracesdk.WithBatchTimeout(OTLPBatchTimeout)),
			tracesdk.WithResource(res),
		)
	}

	return traceProvider, nil
}
