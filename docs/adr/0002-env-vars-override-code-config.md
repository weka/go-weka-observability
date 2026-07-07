# Environment variables always override code configuration

Logger and OTel configuration set in code (functional options like
`WithInfoLevel()`, `WithFileSink()`) is treated as the *default*, and `LOG_*` /
`OTEL_*` environment variables override it unconditionally — the reverse of the
usual "explicit code wins" convention. We chose this (12-factor style) so
operators can change log level, format, or sink of a deployed binary without a
recompile or a code change from the consuming team; the library applies env
overrides automatically inside `CreateLoggerFrom()` and
`SetupOTelSDKWithOptions()` rather than requiring callers to opt in.

## Consequences

- A developer's code-level setting silently loses to an env var present in the
  runtime environment. This is by design — don't "fix" it by making code win.
- Env overrides are applied via `envconfig` with the `LOG` and `OTEL` prefixes
  (`logger/config.go`, `instrumentation/otel_config.go`).
