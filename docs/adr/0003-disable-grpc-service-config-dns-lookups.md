# gRPC service-config DNS TXT lookups disabled by default

The OTLP gRPC trace exporter is created with `grpc.WithDisableServiceConfig()`
by default (`DisableGRPCServiceConfig: true` in `OTelConfig`), deviating from
gRPC's own default of resolving service config via DNS TXT/SRV records. On
networks where those records don't exist — VPNs, corporate DNS — the lookups
stall and caused slow span exports and export timeouts, so we traded away
DNS-delivered service config (which our collectors don't use) for predictable
export latency.

## Consequences

- Do not "fix" this back to gRPC defaults; the slow-DNS failure mode returns.
- The old behavior is restorable per-deployment with
  `OTEL_GRPC_DISABLE_SERVICE_CONFIG=false` (see ADR-0002 for env precedence).
