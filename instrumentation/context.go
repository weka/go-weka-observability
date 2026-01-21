package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/weka/go-weka-observability/logger"
)

// NewContextWithTraceID creates a context with a remote span context from a trace ID string.
// This is useful for propagating trace context from external sources.
//
// Note: The tracer parameter is unused but kept for API compatibility.
func NewContextWithTraceID(ctx context.Context, _ trace.Tracer, traceIDStr string) context.Context {
	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		logger.LogrFromContextOrDefault(ctx).V(VerbosityLevelDebug).Info(
			"invalid trace ID, will use zero trace ID",
			"traceID", traceIDStr,
			"error", err,
		)
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithRemoteSpanContext(ctx, sc)
}

// NewContextWithSpanID creates a context with remote span context from trace and span ID strings.
// This is useful for propagating trace context from external sources.
//
// Note: The tracer parameter is unused but kept for API compatibility.
func NewContextWithSpanID(
	ctx context.Context,
	_ trace.Tracer,
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

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithRemoteSpanContext(ctx, sc)
}
