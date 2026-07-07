# Weka Observability Toolkit

Structured logging and OpenTelemetry tracing for Go services. The distinctive
concept is span *ownership*: whether a piece of code is responsible for ending
the span it holds.

## Language

### Span ownership

**Owned span**:
A span whose lifecycle the current code controls and must explicitly end.
Represented by a `SpanLogger`.
_Avoid_: managed span

**Borrowed span**:
A span created elsewhere that the current code may log through but must not end.
Represented by a `SpanLoggerView`.
_Avoid_: shared span, referenced span

**SpanLogger**:
A handle combining an owned span with a logger. The holder must call `End()`,
typically via `defer`. Returned by the `Create*LogSpan` constructors.
_Avoid_: bare "logger" (collides with the embedded `logr.Logger` and the `logger` package)

**SpanLoggerView**:
A handle combining a borrowed span with a logger. It has no `End()` method, so
the compiler prevents ending a span you don't own. Returned by `CurrentSpanLogger`.
_Avoid_: read-only logger, view — it can still log and mutate the span; it just cannot end it
