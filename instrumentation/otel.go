package instrumentation

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

var otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

// SetupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(
	ctx context.Context, serviceName, serviceVersion string, logger logr.Logger,
) (shutdown func(context.Context) error, err error) {
	if otlpEndpoint == "" {
		return func(ctx context.Context) error { return nil }, nil
	}

	logger.V(logLevelDebug).WithCallDepth(1).Info("Setting up OTel SDK",
		"service", serviceName, "version", serviceVersion)

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTraceProvider(ctx, serviceName, serviceVersion, logger)
	if err != nil {
		return func(ctx context.Context) error { return nil }, err
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

func newResource(serviceName, serviceVersion string) *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	return r
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(ctx context.Context, serviceName, serviceVersion string, logger logr.Logger) (*tracesdk.TracerProvider, error) {
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
			otlptracegrpc.WithTimeout(5*time.Second),
			otlptracegrpc.WithEndpointURL(otlpEndpoint),
		)
		if err != nil {
			logger.Error(err, "failed to create OTLP trace exporter")
			return nil, err
		}

		traceProvider = tracesdk.NewTracerProvider(
			tracesdk.WithBatcher(traceExporter,
				// Default is 5s. Set to 1s for demonstrative purposes.
				tracesdk.WithBatchTimeout(time.Second)),
			tracesdk.WithResource(newResource(serviceName, serviceVersion)),
		)
	}

	return traceProvider, nil
}

func NewContextWithTraceID(ctx context.Context, traceIDStr string) context.Context {
	traceID, _ := trace.TraceIDFromHex(traceIDStr)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
	//retCtx, _ := tracer.Start(ctx, "SharedClusterContext")
	return ctx
}

func NewContextWithSpanID(ctx context.Context, traceIDStr string, spanIdStr string) context.Context {
	traceID, _ := trace.TraceIDFromHex(traceIDStr)
	spanID, _ := trace.SpanIDFromHex(spanIdStr) // Example span ID; typically this would also come from external data

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx = trace.ContextWithRemoteSpanContext(ctx, sc)

	return ctx
}
