# Deprecate old API surface instead of breaking it

When the SpanLogger API replaced the original span/logger functions (PR #23),
the old surface (`GetLogSpan`, `GetLoggerForContext`, `SetupOTelSDK`) was kept
fully working in `instrumentation/deprecations.go` with `Deprecated:` notices
rather than removed. This library is imported by many Weka services on their
own upgrade schedules; a breaking release would force lockstep migrations, so
we pay the cost of maintaining a shim layer indefinitely instead.

## Consequences

- New code must use the replacement API (see the mapping in
  `deprecations.go` and CLAUDE.md's Deprecation Notes); the old functions
  exist only for not-yet-migrated consumers.
- Removing `deprecations.go` requires evidence that no downstream consumer
  still calls it — not just a major-version bump.
