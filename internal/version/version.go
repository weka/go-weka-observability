// Package version provides version information for the go-weka-observability library.
// It includes functionality to retrieve the library version for instrumentation
// and observability purposes, following OpenTelemetry best practices.
//
// See docs/versioning.md for the resolution strategy and instrumentation-scope details.
package version

import (
	"runtime/debug"
	"strings"
)

const (
	// fallbackVersion is the version to return when the library version is not found.
	fallbackVersion = "0.0.0-dev"
	// scopeFallbackName is the scope instrumentation name to return when the module path is not found.
	scopeFallbackName = "github.com/weka/go-weka-observability"
)

// GetInstrumentationVersion returns the go-weka-observability library version
// using Go's built-in module information. This automatically provides
// the correct version for consumers without any CI/CD automation.
//
// The function strips the 'v' prefix from Git tags to return clean semantic versions
// (e.g., Git tag "v1.2.3" returns "1.2.3").
//
// This version identifies the specific release of the go-weka-observability library
// and is used for:
// - OpenTelemetry meter instrumentation scope version
// - OpenTelemetry tracer instrumentation scope version
// - Debugging and correlation in observability tools
// - Issue tracking and version correlation in production
//
// Version sources and their return values (without 'v' prefix):
// - Tagged releases: "1.2.3" (from go get github.com/weka/go-weka-observability@v1.2.3)
// - Latest: "1.2.3" (from go get github.com/weka/go-weka-observability@latest)
// - Main branch: "0.0.0-20240116123456-abcdef123456" (pseudo-version with timestamp and commit)
// - Local development: "0.0.0-dev" (fallback for local builds showing "(devel)")
//
// This approach leverages Go's module system and eliminates the need for
// version injection or CI/CD automation while providing accurate version
// information for all consumption patterns.
func GetInstrumentationVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		version := info.Main.Version

		switch {
		case version == "" || version == "(devel)":
			// Local development builds or empty version
			return fallbackVersion
		case strings.HasPrefix(version, "v"):
			// Tagged versions (v1.2.3) or pseudo-versions (v0.0.0-timestamp-commit)
			trimmed := strings.TrimPrefix(version, "v")
			if trimmed == "" {
				return fallbackVersion
			}

			return trimmed
		case version != "":
			// Non-empty version without 'v' prefix, return as-is
			return version
		default:
			// Empty or unexpected format
			return fallbackVersion
		}
	}

	// Fallback for very old Go versions or build issues
	return fallbackVersion
}

// GetInstrumentationName returns the go-weka-observability library module path
// using Go's built-in module information. This is used as the instrumentation
// library name in OpenTelemetry instrumentation scope.
//
// The module path uniquely identifies this library and is used for:
// - OpenTelemetry tracer instrumentation scope name
// - OpenTelemetry meter instrumentation scope name
// - Identifying traces/metrics created by this library
//
// Module path sources and their return values:
// - Installed via go get: "github.com/weka/go-weka-observability" (from module info)
// - Local development: "github.com/weka/go-weka-observability" (fallback)
//
// This approach leverages Go's module system to automatically provide the
// correct module path without hardcoding.
func GetInstrumentationName() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Path != "" {
			return info.Main.Path
		}
	}

	// Fallback for local development or build issues
	return scopeFallbackName
}
