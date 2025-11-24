package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

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
