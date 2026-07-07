# Two types for span ownership instead of one

We expose an owned span as `SpanLogger` (has `End()`) and a borrowed span as
`SpanLoggerView` (no `End()` method), rather than a single type whose `End()`
no-ops when the span is borrowed. Splitting into two types turns "you don't own
this span" into a compile error instead of a runtime no-op, eliminating a class
of lifecycle bugs (double-end, ending a parent's span from a helper) before the
code runs.

## Considered Options

- **One type with a runtime-guarded `End()`** — smaller surface, but ownership
  violations only surface at runtime (or silently no-op), and nothing stops a
  helper from ending a span it merely borrowed.
- **Two types (chosen)** — the type system enforces ownership; the small amount
  of duplicated surface is absorbed by the unexported `spanLoggerBase`, which
  holds the shared logging/span behavior.

## Consequences

- Callers must know whether they own a span. The `Create*LogSpan` constructors
  return `*SpanLogger`; `CurrentSpanLogger` returns `*SpanLoggerView`.
- Shared behavior lives in the unexported `spanLoggerBase` to avoid duplication
  between the two public types.
