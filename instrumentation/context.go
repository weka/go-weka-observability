package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/weka/go-weka-observability/logger"
)

func NewContextWithTraceID(ctx context.Context, tracer trace.Tracer, traceIDStr string) context.Context {
	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		logger.LogrFromContextOrDefault(ctx).V(VerbosityLevelDebug).Info(
			"invalid trace ID, will use zero trace ID",
			"traceID", traceIDStr,
			"error", err,
		)
	}

	//nolint:ineffassign,staticcheck
	if tracer == nil {
		tracer = Tracer
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
	// retCtx, _ := tracer.Start(ctx, "SharedClusterContext")

	return ctx
}

// NewContextWithSpanID creates a context with remote span context from trace and span ID strings.
// This is useful for propagating trace context from external sources.
func NewContextWithSpanID(
	ctx context.Context,
	tracer trace.Tracer,
	traceIDStr string,
	spanIDStr string,
) context.Context {
	log := logger.LogrFromContextOrDefault(ctx)

	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		log.V(VerbosityLevelDebug).Info("invalid trace ID, will use zero trace ID", "traceID", traceIDStr, "error", err)
	}

	spanID, err := trace.SpanIDFromHex(spanIDStr)
	if err != nil {
		log.V(VerbosityLevelDebug).Info("invalid span ID, will use zero span ID", "spanID", spanIDStr, "error", err)
	}

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
	// retCtx, span := tracer.Start(ctx, spanName)

	return ctx
}
