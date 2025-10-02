package instrumentation

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	zerologger "github.com/weka/go-weka-observability/logger"
)

func init() {
	// default global settings
	zerologr.VerbosityFieldName = ""
	zerologr.NameSeparator = "."
}

type ContextLoggerKey struct{}
type ContextValuesKey struct{}

// SpanLogger is an abstract object that can be used instead of regular loggers and spans
type SpanLogger struct {
	Ctx context.Context
	logr.Logger
	trace.Span
}

// By default, log string in zerolog that uses `caller` will have formart:
// 2024-09-26T00:00:00+00:00 ERR path/to/file.go:217 > Error running some operation error="error text" additional_field=value logger=TopLevelName.NestedLoggerName
// without `caller`:
// 2024-09-26T00:00:00+00:00 ERR Error running some operation error="error text" additional_field=value logger=TopLevelName.NestedLoggerName
// ---
// This function will change the `logger` field to be put instead of `caller`:
// 2024-09-26T00:00:00+00:00 ERR TopLevelName.NestedLoggerName > Error running some operation error="error text" additional_field=value
func NewZerologrWithLoggerNameInsteadCaller() logr.Logger {
	initLogger := zerologger.NewZeroLoggerWithoutCaller()
	zerologr.NameFieldName = "caller"
	return zerologr.New(initLogger)
}

func GetLoggerForContext(ctx context.Context, baseLogger *logr.Logger, name string, keysAndValues ...any) (context.Context, logr.Logger) {
	var logger logr.Logger
	if baseLogger == nil {
		if ctx.Value(ContextLoggerKey{}) != nil {
			logger = ctx.Value(ContextLoggerKey{}).(logr.Logger)
		} else {
			initLogger := zerologger.NewZeroLogger()
			logger = zerologr.New(initLogger)
		}
	} else {
		logger = *baseLogger
	}

	logger = logger.WithValues(keysAndValues...)
	if name != "" {
		logger = logger.WithName(name)
	}
	retCtx := context.WithValue(ctx, ContextLoggerKey{}, logger)
	return retCtx, logger
}

func GetSpanForContext(ctx context.Context, name string, keysAndValues ...any) (context.Context, trace.Span) {
	if Tracer == nil {
		panic("Tracer is not initialized. Call SetupOTelSDK first")
	}
	if name == "" {
		if len(keysAndValues) != 0 {
			panic("When re-using old context it is forbidden to modify span values, as new span is not created")
		}
		span := trace.SpanFromContext(ctx)
		return ctx, span
	}
	ctx, span := Tracer.Start(ctx, name)
	// expand with values saved previously in context
	if ctx.Value(ContextValuesKey{}) != nil {
		keysAndValues = append(keysAndValues, ctx.Value(ContextValuesKey{}).([]any)...)
	}
	spanAttrs := getAttributesFromKeysAndValues(keysAndValues...)
	span.SetAttributes(spanAttrs...)
	ctx = context.WithValue(ctx, ContextValuesKey{}, keysAndValues)
	return ctx, span
}

func (ls *SpanLogger) Enabled(level int) bool {
	return ls.Logger.Enabled()
}

func getAttributesFromKeysAndValues(keysAndValues ...any) []attribute.KeyValue {
	if len(keysAndValues)%2 != 0 {
		return []attribute.KeyValue{}
	}
	attrs := make([]attribute.KeyValue, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		k, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, attributeFromValue(k, keysAndValues[i+1]))
	}
	return attrs
}

// attributeFromValue creates an OpenTelemetry attribute with proper type handling
func attributeFromValue(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	default:
		// Fallback to string representation for unknown types
		return attribute.String(key, fmt.Sprint(v))
	}
}

func (ls *SpanLogger) Info(msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(1).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Debug(msg string, keysAndValues ...any) {
	// logr.V(1) is equivalent to zerolog.DebugLevel
	ls.V(1).WithCallDepth(1).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Printf(msg string, args ...any) {
	ls.WithCallDepth(1).Info(fmt.Sprintf(msg, args...))
}

func (ls *SpanLogger) Errorf(msg string, args ...any) {
	ls.WithCallDepth(1).Error(fmt.Errorf(msg, args...), "")
}

func (ls *SpanLogger) InfoWithStatus(code codes.Code, msg string, keysAnValues ...any) {
	ls.WithCallDepth(1).Info(msg, keysAnValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAnValues...)...)
	ls.AddEvent(msg)
	ls.SetStatus(code, msg)
}

func (ls *SpanLogger) Warn(msg string, keysAndValues ...any) {
	keysAndValues = append(keysAndValues, "level", "warn")
	ls.Logger.WithCallDepth(1).Info(msg, keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
	ls.AddEvent(msg)
}

func (ls *SpanLogger) Error(err error, msg string, keysAndValues ...any) {
	ls.Logger.WithCallDepth(1).Error(err, msg, keysAndValues...)
	ls.RecordError(err)
}

func (ls *SpanLogger) SetError(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(1).Error(err, msg, keysAndValues...)
	ls.SetStatus(codes.Error, msg)
	// TODO: Validate that error is not set yet
}

func (ls *SpanLogger) SetAttributes(attrs ...attribute.KeyValue) {
	if ls.Span != nil && len(attrs) > 0 {
		ls.Span.SetAttributes(attrs...)
	}
}

func (ls *SpanLogger) Fatal(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(1).Error(err, msg, keysAndValues...)
	os.Exit(1)
}

func (ls *SpanLogger) Panic(err error, msg string, keysAndValues ...any) {
	ls.WithCallDepth(1).Error(err, msg, keysAndValues...)
	panic(err)
}

func (ls *SpanLogger) SetValues(keysAndValues ...any) {
	ls.Logger = ls.Logger.WithValues(keysAndValues...)
	ls.SetAttributes(getAttributesFromKeysAndValues(keysAndValues...)...)
}

func GetLogSpan(ctx context.Context, name string, keysAndValues ...any) (context.Context, *SpanLogger, func()) {
	if name == "" && len(keysAndValues) > 0 {
		panic("GetLogSpan must be called with no key/value pairs if name is empty")
	}
	if len(keysAndValues)%2 != 0 {
		panic("WithValues must be called with an even number of arguments")
	}

	ctx, logger := GetLoggerForContext(ctx, nil, name, keysAndValues...)
	ctx, span := GetSpanForContext(ctx, name, keysAndValues...)

	if span != nil {
		traceID := span.SpanContext().TraceID()
		spanID := span.SpanContext().SpanID()
		if traceID.IsValid() && spanID.IsValid() {
			logger = logger.WithValues("trace_id", traceID.String(), "span_id", spanID.String())
		}
	}

	ShutdownFunc := func() {
		if span != nil && name != "" {
			span.End()
			logger.V(2).Info("span finished", "name", name)
		}
	}

	ls := SpanLogger{
		Logger: logger,
		Span:   span,
	}
	// logr.V(2) is equivalent to zerolog.TraceLevel
	logger.V(2).Info(fmt.Sprintf("%s called", name))
	return ctx, &ls, ShutdownFunc
}
