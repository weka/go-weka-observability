# Codebase Concerns

**Analysis Date:** 2026-07-07

## Tech Debt

**Deprecation Maintenance Burden:**
- Issue: `instrumentation/deprecations.go` (449 lines) contains 4 deprecated public APIs (Tracer variable, NewZerologrWithLoggerNameInsteadCaller, GetLoggerForContext, GetSpanForContext, GetLogSpan, SetupOTelSDK) that must be maintained for backward compatibility
- Files: `instrumentation/deprecations.go`, `instrumentation/logspan.go`, `instrumentation/spanlogger.go`
- Impact: Increases code maintenance burden; internal span creation logic is duplicated across old and new APIs; migration paths are complex and require extensive documentation
- Fix approach: Version 2.0 will remove all deprecated APIs. Until then, ensure all new features use SpanLogger API exclusively. Add deprecation warnings to logs when old APIs are used.

**Internal Helper Functions Lack Godoc:**
- Issue: Private helper functions in `instrumentation/logspan.go` (createChildSpan, createRootSpanInternal, createSpanShutdownFunc, getOrCreateLogger, enrichLogger, addTraceIDsIfValid) lack proper godoc comments explaining parameters and return values
- Files: `instrumentation/logspan.go` (lines 93-287)
- Impact: Future maintainers struggle to understand span lifecycle logic; refactoring is risky
- Fix approach: Add comprehensive godoc comments to all exported-length functions (those longer than simple getters). Focus on: parameters, return values, thread safety, side effects.

**Incomplete TODO in SetError Method:**
- Issue: `instrumentation/spanlogger.go:136` has TODO comment "Validate that error is not set yet" - SetError() doesn't verify the span wasn't already marked as error
- Files: `instrumentation/spanlogger.go` (line 136)
- Impact: If SetError is called twice, the second call silently overwrites the first error status, losing error history in traces
- Fix approach: Track error state on the span (requires careful design to avoid breaking the OpenTelemetry span interface). Consider: (1) checking span status before setting, (2) adding test case for double SetError, (3) documenting idempotency guarantee

## Known Bugs

**Panic-Based Argument Validation:**
- Symptoms: Calls to CreateLogSpan, CreateRootLogSpan, GetLogSpan, and WithValues with odd-number keysAndValues cause immediate panic
- Files: `instrumentation/spanlogger.go` (lines 278, 336), `instrumentation/logspan.go` (lines 222, 225)
- Trigger: Call any span creation function with odd-length keysAndValues: `CreateLogSpan(ctx, "op", "key")` without value
- Workaround: Always pass even-length keysAndValues; use linters to catch violations before runtime
- Issue: Panics are unrecoverable at runtime and difficult to debug in production. Better to return errors or detect at code review time.

## Security Considerations

**Global State via Singletons:**
- Risk: Three module-level singletons manage global state: `globalTracerCache`, `globalZerologrInit`, `globalPropagatorInit`. All use sync.Once for thread-safe initialization.
- Files: `instrumentation/tracer.go` (line 17), `instrumentation/logspan.go` (line 61), `instrumentation/oteltest/oteltest.go` (line 47)
- Current mitigation: sync.Once ensures one-time initialization; RWMutex protects tracer cache; test setup helpers isolate context-based tracers
- Recommendations: (1) Document that these globals are NOT safe to swap after initialization (affects all goroutines). (2) Add lint rule to prevent direct global assignments. (3) For tests requiring different tracers, always use `oteltest.SetupTester()` (context injection) not `SetupTesterWithProvider()` (provider swap).

**Global Provider Swap in Tests:**
- Risk: `instrumentation/oteltest/oteltest.go:125` "WARNING: This swaps the global TracerProvider and TextMapPropagator which are shared across all goroutines. Parallel tests will interfere with each other"
- Files: `instrumentation/oteltest/oteltest.go` (SetupTesterWithProvider), line 125
- Current mitigation: Documentation warns against t.Parallel() with SetupTesterWithProvider; SetupTester() uses context-based isolation instead
- Recommendations: (1) Rename SetupTesterWithProvider to SetupTesterSequential to make ordering requirement explicit. (2) Add build tags to prevent accidental parallel use. (3) Audit all tests using SetupTesterWithProvider and convert to SetupTester where possible.

## Performance Bottlenecks

**DNS TXT Record Lookup Stall:**
- Problem: gRPC's default service-config resolution via DNS TXT records stalls on corporate/VPN networks where those records don't exist
- Files: `instrumentation/otel.go`, `docs/adr/0003-disable-grpc-service-config-dns-lookups.md`
- Cause: grpc default config, now disabled via `DisableGRPCServiceConfig: true` in `OTelConfig`
- Improvement path: Already fixed via ADR-0003. Document that reverting to gRPC defaults (via `OTEL_GRPC_DISABLE_SERVICE_CONFIG=false`) will trigger slow DNS failures on corporate networks. This is a known tradeoff: gRPC docs say service config provides performance hints, but for observability collectors we don't use those hints.

**Span Attribute Conversion Overhead:**
- Problem: Every keysAndValues passed to CreateLogSpan is converted to OpenTelemetry attributes twice: once for logging, once for span attributes
- Files: `instrumentation/logspan.go` (lines 154-218: getAttributesFromKeysAndValues), `instrumentation/spanlogger.go` (lines 80, 107, 116, 124, 142, 161)
- Cause: SpanLogger converts in Info/Warn/Error/SetAttributes; span creation converts separately; no deduplication
- Improvement path: Profile to verify this is actual bottleneck (may be negligible). If significant: pre-convert keysAndValues once during span creation, store in struct, reuse across methods. Trade-off: adds struct fields.

**Tracer Cache Invalidation on Provider Swap:**
- Problem: `instrumentation/tracer.go` uses RWMutex double-check pattern that requires write lock when provider changes
- Files: `instrumentation/tracer.go` (lines 70-95)
- Cause: Necessary cost of provider change detection for test isolation
- Improvement path: Already optimized: reads are fast (~45-75ns in production where provider never changes), writes only on detection (~1-10μs, rare). Document performance expectations in godoc.

## Fragile Areas

**Span Lifecycle with Context Propagation:**
- Files: `instrumentation/spanlogger.go`, `instrumentation/logspan.go`, `instrumentation/tracer.go`
- Why fragile: SpanLogger owns a shutdown function that calls span.End(); if End() is not called (caller forgets defer), the span never closes and batch exporter may drop it. Additionally, CreateLogSpan returns a new context with updated span; if caller ignores the context and passes old context to children, trace relationships break.
- Safe modification: (1) Ensure all calls to Create*LogSpan are followed immediately by `defer logger.End()`. (2) Always use the returned context, never the old one. (3) Add tests that verify span closure via recorder assertions. (4) Consider adding a finalizer that warns if logger is garbage collected without End() call (warning only, no hard guarantee).
- Test coverage: `instrumentation/logspan_test.go` and `instrumentation/spanlogger_test.go` cover happy paths; edge case: unused returned contexts and missing defer calls are not tested.

**Deprecated API Pathways:**
- Files: `instrumentation/deprecations.go` (449 lines)
- Why fragile: Old GetLogSpan, GetSpanForContext, GetLoggerForContext APIs internally delegate to new APIs via wrapper functions. Changes to internal helper functions (createChildSpan, enrichLogger) must remain backward-compatible with both paths.
- Safe modification: (1) Any change to helpers in logspan.go must be tested with both new (CreateLogSpan) and old (GetLogSpan) entry points. (2) Keep old code paths unchanged as long as possible. (3) Add regression tests that call deprecated functions alongside new functions in same test.
- Test coverage: Deprecated APIs have ~50 lines of test coverage in logspan_test.go; new APIs have ~850 lines. Risk: breaking changes to helpers may not be caught by old API tests.

**Global Zerologr Initialization:**
- Files: `instrumentation/logspan.go:75` (initZerologrDefaults), uses sync.Once
- Why fragile: Sets zerologr.VerbosityFieldName and zerologr.NameSeparator globally. If multiple packages initialize zerologr independently with different settings, sync.Once guarantees first wins, later calls are silently ignored.
- Safe modification: (1) Call initZerologrDefaults explicitly in SetupOTelSDK/SetupOTelSDKWithOptions before creating first span. (2) Document that zerologr settings must be consistent across application. (3) Consider moving into a public Init() function instead of lazy init.
- Test coverage: Tested via logger tests; should add explicit test for idempotence.

## Scaling Limits

**Span Exporter Connection Pool:**
- Current capacity: gRPC exporter maintains single connection (http/2 multiplexed), batch size 512 spans, timeout 5s
- Files: `instrumentation/otel.go` (lines 199, 204), `instrumentation/otel.go:447` (newTraceProvider)
- Limit: On high-throughput systems (>10k spans/sec), single connection may become bottleneck; batch timeout may cause dropped spans if collector is unavailable
- Scaling path: (1) Increase OTLPBatchTimeout and batch size via OTelConfig options (not yet exposed via functional options, would require API addition). (2) gRPC http/2 allows multiplexing; bottleneck is collector availability, not client library. (3) Test with realistic load (e.g., benchmark at 10k spans/sec with collector unavailable).

## Dependencies at Risk

**OpenTelemetry SDK Coupling:**
- Risk: Direct dependency on `go.opentelemetry.io/otel/sdk` and `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` means SDK API changes break the code
- Files: `instrumentation/otel.go` imports all OTel SDK packages
- Impact: OTel SDK is pre-v1.0 in some components; breaking changes possible in minor releases
- Migration plan: (1) Monitor OTel release notes for breaking changes. (2) Pin to specific minor versions in go.mod until SDK stabilizes. (3) Wrap OTel types in thin adapter layer to reduce coupling (lower priority, do if SDK instability continues).

**gRPC Service Config Workaround:**
- Risk: Code disables gRPC service config (`grpc.WithDisableServiceConfig()`) due to DNS lookup stalls. If gRPC removes this option, must find alternative solution.
- Files: `instrumentation/otel.go`, `instrumentation/otel_config.go` (DisableGRPCServiceConfig flag)
- Impact: Losing this option would require either: (1) accepting slow DNS lookups, or (2) migrating to HTTP exporter instead of gRPC
- Migration plan: Document decision in ADR-0003. If gRPC removes option, switch to HTTP exporter via `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`.

## Missing Critical Features

**No Error Deduplication on Spans:**
- Problem: SetError() can be called multiple times on same span; each call overwrites previous. No way to record multiple errors in single span.
- Blocks: Error tracking that needs to correlate multiple failures per operation
- Recommendation: Add optional ErrorChain type that tracks sequence of errors. Low priority unless use case emerges.

**No Span Link Management at API Level:**
- Problem: Can only create span links via CreateLogSpanWithOptions + trace.WithLinks(); no convenience method like CreateLinkedLogSpan
- Blocks: Simplified trace linking patterns (e.g., linking multiple input traces to single output)
- Recommendation: Add CreateLinkedLogSpan convenience function if span linking becomes common pattern.

## Test Coverage Gaps

**Private Helper Functions Not Directly Tested:**
- Untested area: Private functions in `instrumentation/logspan.go` and `instrumentation/logspan_internal.go` (if it exists) are tested only indirectly via public API tests
- Files: `instrumentation/logspan.go` (lines 93-287: createChildSpan, createRootSpanInternal, getCurrentSpan, getAttributesFromKeysAndValues, attributeFromValue, attributeFromScalar, attributeFromSlice, validateGetLogSpanArgs, getOrCreateLogger, enrichLogger, addTraceIDsIfValid, createSpanShutdownFunc, logOperationStart)
- Risk: Mutations to these functions may not be caught if public API tests don't exercise the code path
- Priority: Medium. Add unit tests for: (1) attribute conversion edge cases (nil values, empty slices, maps). (2) trace ID enrichment for different span states (valid, no-op, invalid). (3) context value merging in createChildSpan.

**Concurrent Span Creation Not Tested:**
- Untested area: Creating multiple spans concurrently and verifying trace relationships remain correct
- Files: `instrumentation/tracer.go`, `instrumentation/spanlogger.go`
- Risk: Race condition where concurrent span creation to same parent causes incorrect span ordering or missing parent links
- Priority: High. Add test using -race flag and t.Parallel() to verify no races. Use real tracer (not mock) to catch synchronization bugs in tracer cache.

**Error Path in SetupOTelSDK Not Fully Covered:**
- Untested area: Error cases in setupOTelSDKInternal (lines 348-401): endpoint empty, tracer provider creation failure, shutdown failure
- Files: `instrumentation/otel.go`
- Risk: Shutdown function may not properly clean up resources if error occurs during setup
- Priority: Medium. Add tests: (1) empty endpoint returns no-op shutdown, (2) invalid endpoint returns error and cleanup, (3) shutdown handles flush/ForceFlush errors gracefully.

**Logger Context Retrieval Edge Cases:**
- Untested area: getOrCreateLogger in `instrumentation/logspan.go:231` may return default logger if context has no logger. Behavior if context is nil or canceled.
- Files: `instrumentation/logspan.go` (line 231)
- Risk: Creating spans in already-canceled context succeeds silently instead of propagating cancellation
- Priority: Medium. Add tests: (1) span creation with canceled context, (2) span creation with nil context (if supported), (3) verify cancel propagation.

---

*Concerns audit: 2026-07-07*
