package version_test

import (
	"runtime/debug"
	"testing"

	"github.com/weka/go-weka-observability/internal/version"
)

func TestGetInstrumentationVersion(t *testing.T) {
	// Debug information for CI troubleshooting
	if info, ok := debug.ReadBuildInfo(); ok {
		t.Logf("BuildInfo available - Path: %s, Version: %q", info.Path, info.Main.Version)
	} else {
		t.Logf("BuildInfo not available")
	}

	// Test that the function returns a valid version string
	v := version.GetInstrumentationVersion()

	if v == "" {
		t.Fatalf("GetInstrumentationVersion() returned empty string - this should never happen")
	}

	// In local development, we expect "0.0.0-dev"
	// When consumed via go get, we expect actual version
	t.Logf("Version returned: %q", v)

	// For local development builds, we expect "0.0.0-dev"
	if v != "0.0.0-dev" {
		t.Logf("Non-dev version detected: %s (this is expected when consumed via go get)", v)
	}
}

func TestGetInstrumentationVersionNonEmpty(t *testing.T) {
	// Ensure function always returns a non-empty string
	v := version.GetInstrumentationVersion()
	if len(v) == 0 {
		t.Fatalf("GetInstrumentationVersion() must never return empty string")
	}

	// Verify it returns a reasonable value
	if v != "0.0.0-dev" && len(v) < 3 {
		t.Logf("Warning: Version seems unusually short: %q", v)
	}
}

func TestGetInstrumentationName(t *testing.T) {
	// Debug information for CI troubleshooting
	if info, ok := debug.ReadBuildInfo(); ok {
		t.Logf("BuildInfo available - Path: %s", info.Main.Path)
	} else {
		t.Logf("BuildInfo not available")
	}

	// Test that the function returns a valid module path
	name := version.GetInstrumentationName()

	if name == "" {
		t.Fatalf("GetInstrumentationName() returned empty string - this should never happen")
	}

	// Expected module path
	expectedPath := "github.com/weka/go-weka-observability"
	if name != expectedPath {
		t.Logf("Unexpected module path: got %q, expected %q", name, expectedPath)
	}

	t.Logf("Module path returned: %q", name)
}

func TestGetInstrumentationNameNonEmpty(t *testing.T) {
	// Ensure function always returns a non-empty string
	name := version.GetInstrumentationName()
	if len(name) == 0 {
		t.Fatalf("GetInstrumentationName() must never return empty string")
	}

	// Verify it's a reasonable module path format
	if len(name) < 5 {
		t.Fatalf("Module path seems too short: %q", name)
	}
}
