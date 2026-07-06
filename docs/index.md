# Repo Map

**Logging**
- [logger-configuration-api.md](logger-configuration-api.md) — logger config, rotation, and env overrides; `Config`, `CreateLogger`, `CreateLoggerFrom`
- [logger-initialization-migration.md](logger-initialization-migration.md) — migrating off deprecated `GetLoggerForContext`; `LogrFromContextOrDefault`, `ContextWithLogr`

**Tracing**
- [instrumentation-configuration-api.md](instrumentation-configuration-api.md) — OTel SDK setup and env-driven config; `SetupOTelSDKWithOptions`, `OTelConfig`
- [spanlogger-api.md](spanlogger-api.md) — combined logging+tracing span API; `SpanLogger`, `SpanLoggerView`, `CreateLogSpan`
- [trace-management.md](trace-management.md) — tracer resolution and provider-swap caching; `GetTracer`, `ContextWithTracer`

**Architecture**
- [versioning.md](versioning.md) — automatic module-version and instrumentation-scope resolution; `GetInstrumentationVersion`
