package instrumentation

import (
	"context"
	"errors"
	"os"
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
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

var (
	Tracer trace.Tracer

	otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
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
func SetupOTelSDK(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, keysAndValues ...any) (shutdown func(context.Context) error, err error) {
	logger.V(VerbosityLevelDebug).WithCallDepth(CallDepthOffset).Info("Setting up OTel SDK", "service", serviceName, "version", serviceVersion)
	Tracer = otel.Tracer(serviceName)

	if otlpEndpoint == "" {
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
	tracerProvider, err := newTraceProvider(ctx, serviceName, serviceVersion, logger, keysAndValues...)
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

func newTraceProvider(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger, keysAndValues ...any) (*tracesdk.TracerProvider, error) {
	logger.Info("Setting up OTel trace provider", "service", serviceName, "version", serviceVersion)
	var traceProvider *tracesdk.TracerProvider

	if otlpEndpoint != "" {
		logger.Info("OTLP endpoint set", "endpoint", otlpEndpoint)

		securityOption := otlptracegrpc.WithInsecure()
		if strings.Contains(otlpEndpoint, "https://") {
			securityOption = otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
		}
		traceExporter, err := otlptracegrpc.New(ctx,
			securityOption,
			otlptracegrpc.WithTimeout(OTLPExporterTimeout),
			otlptracegrpc.WithEndpointURL(otlpEndpoint),
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
