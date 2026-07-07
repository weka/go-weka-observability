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
- [../CONTEXT.md](../CONTEXT.md) — domain glossary; span-ownership vocabulary (owned/borrowed, `SpanLogger`/`SpanLoggerView`)
- [adr/index.md](adr/index.md) — architectural decision records index

**Codebase Map** (generated snapshot, 2026-07-07)
- [codebase/STACK.md](codebase/STACK.md) — languages, runtime, frameworks, dependencies
- [codebase/ARCHITECTURE.md](codebase/ARCHITECTURE.md) — patterns, layers, data flow, entry points
- [codebase/STRUCTURE.md](codebase/STRUCTURE.md) — directory layout and naming conventions
- [codebase/CONVENTIONS.md](codebase/CONVENTIONS.md) — code style, patterns, error handling
- [codebase/TESTING.md](codebase/TESTING.md) — test frameworks, structure, coverage
- [codebase/INTEGRATIONS.md](codebase/INTEGRATIONS.md) — external services and exporters
- [codebase/CONCERNS.md](codebase/CONCERNS.md) — tech debt, fragile areas, known issues
