package instrumentation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/weka/go-weka-observability/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

var (
	Tracer trace.Tracer
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

// SetupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
// Additional resource attributes can be provided as key-value pairs.
//
// Deprecated: Use SetupOTelSDKFrom or SetupOTelSDKWithOptions instead.
// This function maintains backward compatibility but doesn't allow endpoint configuration via API.
//
// Migration examples:
//
// Old:
//
//	shutdown, err := instrumentation.SetupOTelSDK(ctx, "service", "v1", logger, "key", "value")
//
// New (functional options):
//
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "service", "v1", logger,
//	    instrumentation.WithDefaultOTLPEndpoint("http://localhost:4317"),
//	    instrumentation.WithResourceAttributes("key", "value"),
//	)
//
// New (explicit config):
//
//	config := instrumentation.NewDefaultOTelConfigWithEnvOverrides()
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "service", "v1", logger, config, "key", "value")
func SetupOTelSDK(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, keysAndValues ...any) (shutdown func(context.Context) error, err error) {
	return SetupOTelSDKWithOptions(
		ctx, serviceName, serviceVersion, logger,
		WithResourceAttributes(keysAndValues...),
	)
}

// SetupOTelSDKFrom bootstraps the OpenTelemetry pipeline with explicit configuration.
// If it does not return an error, make sure to call shutdown for proper cleanup.
//
// This function follows the same pattern as logger.CreateLoggerFrom - you provide a config
// that can include defaults overridden by environment variables.
//
// Example with environment defaults:
//
//	config := instrumentation.NewDefaultOTelConfigWithEnvOverrides()
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "my-service", "v1.0.0", logger, config, "key", "value")
//
// Example with custom defaults that can be overridden by env:
//
//	config := instrumentation.OTelConfig{
//	    Endpoint: "http://default-collector:4317",  // This is the DEFAULT
//	}
//	config = instrumentation.NewOTelConfigFromEnv(config)  // Env can override
//	shutdown, err := instrumentation.SetupOTelSDKFrom(ctx, "my-service", "v1.0.0", logger, config, "key", "value")
func SetupOTelSDKFrom(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, config OTelConfig, keysAndValues ...any) (shutdown func(context.Context) error, err error) {
	logger.V(VerbosityLevelDebug).WithCallDepth(CallDepthOffset).Info("Setting up OTel SDK", "service", serviceName, "version", serviceVersion)

	// Merge provided keysAndValues with config's ResourceAttributes
	// Copy ResourceAttributes to avoid mutating the caller's config
	if len(keysAndValues) > 0 {
		newAttrs := make([]any, len(config.ResourceAttributes))
		copy(newAttrs, config.ResourceAttributes)
		config.ResourceAttributes = append(newAttrs, keysAndValues...)
	}

	return setupOTelSDKInternal(ctx, serviceName, serviceVersion, logger, config)
}

// SetupOTelSDKWithOptions bootstraps the OpenTelemetry pipeline with functional options.
// If it does not return an error, make sure to call shutdown for proper cleanup.
//
// This function follows the same pattern as logger.CreateLogger - functional options
// are applied to defaults, then environment variables can override.
//
// Example usage:
//
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "my-service", "v1.0.0", logger,
//	    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
//	    instrumentation.WithResourceAttributes("environment", "production"),
//	)
//	if err != nil {
//	    return err
//	}
//	defer shutdown(ctx)
func SetupOTelSDKWithOptions(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, opts ...OTelOption) (shutdown func(context.Context) error, err error) {
	logger.V(VerbosityLevelDebug).WithCallDepth(CallDepthOffset).Info("Setting up OTel SDK", "service", serviceName, "version", serviceVersion)

	// Start with defaults
	config := DefaultOTelConfig()

	// Apply functional options to set defaults
	for _, opt := range opts {
		opt(&config)
	}

	// Environment variables can override the defaults set by options
	config = NewOTelConfigFromEnv(config)

	return setupOTelSDKInternal(ctx, serviceName, serviceVersion, logger, config)
}

// setupOTelSDKInternal contains the actual OpenTelemetry SDK initialization logic.
// This is extracted to be reused by both SetupOTelSDK and SetupOTelSDKWithOptions.
func setupOTelSDKInternal(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, config OTelConfig) (shutdown func(context.Context) error, err error) {
	// Create tracer with library name and version (instrumentation scope)
	// This identifies the go-weka-observability library itself, not the user's service
	// Both name and version are automatically determined from Go module information
	Tracer = otel.Tracer(
		version.GetInstrumentationName(),
		trace.WithInstrumentationVersion(version.GetInstrumentationVersion()),
	)

	if config.Endpoint == "" {
		logger.V(VerbosityLevelInfo).WithCallDepth(CallDepthOffset).Info("No OTLP endpoint configured - traces will not be exported")
		return func(ctx context.Context) error {
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
	tracerProvider, err := newTraceProvider(ctx, serviceName, serviceVersion, config.Endpoint, logger, config.ResourceAttributes...)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	otel.SetTracerProvider(tracerProvider)

	return func(ctx context.Context) error {
		err = tracerProvider.ForceFlush(context.Background())
		if err != nil {
			logger.Error(err, "failed to flush traces")
		}
		return tracerProvider.Shutdown(ctx)
	}, err
}

func newResource(ctx context.Context, serviceName, serviceVersion string, keysAndValues ...any) (*resource.Resource, error) {
	// Start with the basic required attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(serviceVersion),
	}

	// Add any additional resource attributes provided using the existing helper function
	additionalAttrs := getAttributesFromKeysAndValues(keysAndValues...)
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

func newTraceProvider(ctx context.Context, serviceName, serviceVersion, endpoint string, logger logr.Logger, keysAndValues ...any) (*tracesdk.TracerProvider, error) {
	logger.Info("Setting up OTel trace provider", "service", serviceName, "version", serviceVersion)
	var traceProvider *tracesdk.TracerProvider

	if endpoint != "" {
		logger.Info("OTLP endpoint set", "endpoint", endpoint)

		securityOption := otlptracegrpc.WithInsecure()
		if strings.Contains(endpoint, "https://") {
			securityOption = otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
		}
		traceExporter, err := otlptracegrpc.New(ctx,
			securityOption,
			otlptracegrpc.WithTimeout(OTLPExporterTimeout),
			otlptracegrpc.WithEndpointURL(endpoint),
		)
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

func NewContextWithTraceID(ctx context.Context, tracer trace.Tracer, traceIDStr string) context.Context {
	traceID, _ := trace.TraceIDFromHex(traceIDStr)

	//nolint:ineffassign,staticcheck
	if tracer == nil {
		tracer = Tracer
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
	//retCtx, _ := tracer.Start(ctx, "SharedClusterContext")
	return ctx
}

func NewContextWithSpanID(ctx context.Context, tracer trace.Tracer, traceIDStr string, spanIdStr string) context.Context {
	traceID, _ := trace.TraceIDFromHex(traceIDStr)
	spanID, _ := trace.SpanIDFromHex(spanIdStr) // Example span ID; typically this would also come from external data

	//nolint:ineffassign,staticcheck
	if tracer == nil {
		tracer = Tracer
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
	//retCtx, span := tracer.Start(ctx, spanName)
	return ctx
}
