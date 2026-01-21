package instrumentation_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weka/go-weka-observability/instrumentation"
)

// cleanupEnvVar is a helper function to unset an environment variable and log any errors
func cleanupEnvVar(t *testing.T, key string) {
	if err := os.Unsetenv(key); err != nil {
		t.Logf("Failed to unset environment variable %s: %v", key, err)
	}
}

func TestDefaultOTelConfig(t *testing.T) {
	config := instrumentation.DefaultOTelConfig()

	assert.Empty(t, config.Endpoint, "Default endpoint should be empty")
	assert.Nil(t, config.ResourceAttributes, "Default resource attributes should be nil")
}

func TestNewOTelConfigFromEnv_NoEnvVars(t *testing.T) {
	// Ensure no env var is set
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	defaultConfig := instrumentation.DefaultOTelConfig()
	config := instrumentation.NewOTelConfigFromEnv(defaultConfig)

	assert.Empty(t, config.Endpoint, "Should use default endpoint when no env var")
}

func TestNewOTelConfigFromEnv_WithEnvVar(t *testing.T) {
	expectedEndpoint := "http://test-collector:4317"
	require.NoError(t, os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", expectedEndpoint))
	defer cleanupEnvVar(t, "OTEL_EXPORTER_OTLP_ENDPOINT")

	defaultConfig := instrumentation.DefaultOTelConfig()
	config := instrumentation.NewOTelConfigFromEnv(defaultConfig)

	assert.Equal(t, expectedEndpoint, config.Endpoint, "Should override with env var")
}

func TestNewOTelConfigFromEnv_EnvOverridesDefault(t *testing.T) {
	expectedEndpoint := "http://env-collector:4317"
	require.NoError(t, os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", expectedEndpoint))
	defer cleanupEnvVar(t, "OTEL_EXPORTER_OTLP_ENDPOINT")

	defaultConfig := instrumentation.OTelConfig{
		Endpoint: "http://default-collector:4317",
	}
	config := instrumentation.NewOTelConfigFromEnv(defaultConfig)

	assert.Equal(t, expectedEndpoint, config.Endpoint, "Env var should override default")
}

func TestNewDefaultOTelConfigWithEnvOverrides(t *testing.T) {
	expectedEndpoint := "http://collector:4317"
	require.NoError(t, os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", expectedEndpoint))
	defer cleanupEnvVar(t, "OTEL_EXPORTER_OTLP_ENDPOINT")

	config := instrumentation.NewDefaultOTelConfigWithEnvOverrides()

	assert.Equal(t, expectedEndpoint, config.Endpoint)
}

func TestWithDefaultOTLPEndpoint(t *testing.T) {
	expectedEndpoint := "http://custom-collector:4317"
	config := instrumentation.DefaultOTelConfig()

	option := instrumentation.WithDefaultOTLPEndpoint(expectedEndpoint)
	option(&config)

	assert.Equal(t, expectedEndpoint, config.Endpoint)
}

func TestWithResourceAttributes(t *testing.T) {
	expectedAttrs := []any{"key1", "value1", "key2", "value2"}
	config := instrumentation.DefaultOTelConfig()

	option := instrumentation.WithResourceAttributes(expectedAttrs...)
	option(&config)

	assert.Equal(t, expectedAttrs, config.ResourceAttributes)
}

func TestConfigPriority_EnvOverridesOption(t *testing.T) {
	envEndpoint := "http://env-collector:4317"
	optionEndpoint := "http://option-collector:4317"

	require.NoError(t, os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", envEndpoint))
	defer cleanupEnvVar(t, "OTEL_EXPORTER_OTLP_ENDPOINT")

	// Start with defaults
	config := instrumentation.DefaultOTelConfig()

	// Apply functional option to set default
	option := instrumentation.WithDefaultOTLPEndpoint(optionEndpoint)
	option(&config)
	assert.Equal(t, optionEndpoint, config.Endpoint, "Option should set default value")

	// Environment variable overrides the option
	config = instrumentation.NewOTelConfigFromEnv(config)
	assert.Equal(t, envEndpoint, config.Endpoint, "Env var should override option default")
}

func TestMultipleOptions(t *testing.T) {
	config := instrumentation.DefaultOTelConfig()

	endpointOption := instrumentation.WithDefaultOTLPEndpoint("http://collector:4317")
	attrsOption := instrumentation.WithResourceAttributes("env", "test", "region", "us-west")

	endpointOption(&config)
	attrsOption(&config)

	assert.Equal(t, "http://collector:4317", config.Endpoint)
	assert.Equal(t, []any{"env", "test", "region", "us-west"}, config.ResourceAttributes)
}

func TestSetupOTelSDKFrom_DoesNotMutateConfig(t *testing.T) {
	// This test verifies that SetupOTelSDKFrom does not modify the caller's config
	// when merging keysAndValues with ResourceAttributes

	originalAttrs := []any{"original_key", "original_value"}
	config := instrumentation.OTelConfig{
		Endpoint:           "", // Empty endpoint to skip actual setup
		ResourceAttributes: originalAttrs,
	}

	// Create a snapshot of the original values for comparison
	originalKey := config.ResourceAttributes[0]
	originalValue := config.ResourceAttributes[1]

	// Call SetupOTelSDKFrom with additional keysAndValues
	ctx := context.Background()
	logger := logr.Discard()
	_, _ = instrumentation.SetupOTelSDKFrom(
		ctx, "test-service", "v1.0.0", logger, config,
		"additional_key", "additional_value",
	)

	// Verify the original config's ResourceAttributes were not mutated
	assert.Len(t, config.ResourceAttributes, 2,
		"Original ResourceAttributes should still have 2 elements")

	assert.Equal(t, originalKey, config.ResourceAttributes[0],
		"First element should not be modified")

	assert.Equal(t, originalValue, config.ResourceAttributes[1],
		"Second element should not be modified")
}
