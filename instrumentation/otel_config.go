package instrumentation

import (
	"log/slog"

	"github.com/kelseyhightower/envconfig"
)

// OTelConfig configures OpenTelemetry SDK behavior.
// Follows the same pattern as logger.Config with environment variable overrides.
type OTelConfig struct {
	// Endpoint is the OTLP exporter endpoint (e.g., "http://localhost:4317")
	// Empty string means no traces will be exported
	Endpoint string `envconfig:"EXPORTER_OTLP_ENDPOINT"`

	// ResourceAttributes are additional key-value pairs attached to all spans
	ResourceAttributes []any
}

// OTelOption configures OTelConfig via functional options pattern
type OTelOption func(*OTelConfig)

// DefaultOTelConfig returns default OpenTelemetry configuration.
// By default, no endpoint is set, meaning traces will not be exported.
func DefaultOTelConfig() OTelConfig {
	return OTelConfig{
		Endpoint:           "",
		ResourceAttributes: nil,
	}
}

// NewOTelConfigFromEnv creates OTelConfig with environment overrides.
// Environment variables with OTEL_* prefix can override default values.
// If environment variable processing fails, defaults are used as fallback.
//
// Priority order:
//  1. Functional options (WithOTLPEndpoint, etc.) - Applied after this function
//  2. Environment variables (OTEL_EXPORTER_OTLP_ENDPOINT) - Applied by this function
//  3. Default config values (passed as defaultConfig parameter)
func NewOTelConfigFromEnv(defaultConfig OTelConfig) OTelConfig {
	if err := envconfig.Process("OTEL", &defaultConfig); err != nil {
		slog.Warn("failed to process OTEL_* environment variables, using defaults",
			"error", err,
			"defaults", defaultConfig)
	}

	return defaultConfig
}

// NewDefaultOTelConfigWithEnvOverrides is a convenience wrapper for NewOTelConfigFromEnv
// with DefaultOTelConfig as the base configuration.
func NewDefaultOTelConfigWithEnvOverrides() OTelConfig {
	return NewOTelConfigFromEnv(DefaultOTelConfig())
}

// WithDefaultOTLPEndpoint sets the OTLP endpoint to use when OTEL_EXPORTER_OTLP_ENDPOINT
// environment variable is not set. If neither is set, traces will not be exported.
//
// The OTEL_EXPORTER_OTLP_ENDPOINT environment variable always takes precedence if set,
// regardless of whether you use this option.
//
// Example:
//
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "my-service", "v1.0.0", logger,
//	    instrumentation.WithDefaultOTLPEndpoint("http://otel-collector:4317"),
//	)
//	// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it will be used instead of the default above
func WithDefaultOTLPEndpoint(endpoint string) OTelOption {
	return func(c *OTelConfig) {
		c.Endpoint = endpoint
	}
}

// WithResourceAttributes sets additional resource attributes for all spans.
// Resource attributes are metadata attached to all telemetry from this service.
//
// Example:
//
//	shutdown, err := instrumentation.SetupOTelSDKWithOptions(
//	    ctx, "my-service", "v1.0.0", logger,
//	    instrumentation.WithResourceAttributes("environment", "production", "region", "us-west"),
//	)
func WithResourceAttributes(keysAndValues ...any) OTelOption {
	return func(c *OTelConfig) {
		c.ResourceAttributes = keysAndValues
	}
}
