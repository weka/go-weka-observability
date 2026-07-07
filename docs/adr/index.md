# ADR Index

Architectural decision records — one file per decision, sequentially numbered.
Add new ADRs here as they land.

- [0001](0001-span-ownership-two-types.md) — two types for span ownership; `SpanLogger` (owned, has `End()`) vs `SpanLoggerView` (borrowed, no `End()`)
- [0002](0002-env-vars-override-code-config.md) — environment variables always override code configuration (12-factor)
- [0003](0003-disable-grpc-service-config-dns-lookups.md) — gRPC service-config DNS TXT lookups disabled by default (slow-DNS failure mode)
- [0004](0004-deprecate-dont-break.md) — deprecated API kept working in `deprecations.go` instead of breaking consumers
