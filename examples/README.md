# Examples

This directory contains example programs demonstrating various features of the go-weka-observability library. Each example is a standalone Go program that can be run directly.

## Available Examples

### 1. Basic Example (`basic/`)
A simple example demonstrating basic logging and tracing functionality with nested function calls.

**Run with:**
```bash
go run ./examples/basic
```

**Features demonstrated:**
- Basic SpanLogger usage
- Nested span creation
- Context propagation between functions
- OpenTelemetry SDK setup

### 2. HTTP Tracing Example (`http_tracing/`)
A comprehensive example showing HTTP client/server communication with distributed tracing.

**Run with:**
```bash
go run ./examples/http_tracing
```

**Features demonstrated:**
- HTTP server with tracing middleware
- HTTP client with automatic trace propagation
- Distributed tracing across service boundaries
- Trace context extraction and injection
- Multiple HTTP endpoints with different complexity levels

## Environment Variables

All examples support the following environment variables for configuration:

- `LOG_LEVEL`: Set to "0" for trace level, "1" for debug, "2" for info (default varies by example)
- `LOG_FORMAT`: Set to "json", "plain", or "raw" (default varies by example)
- `LOG_CALLER_DIR_LVL`: Number of directory levels to show in caller info (default: "1")
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint (optional)

## Running with OpenTelemetry Collector

To export traces to an OpenTelemetry collector, set the endpoint environment variable:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
go run ./examples/basic
```

## Example Structure

Each example is organized as a standalone Go main package in its own directory:

```
examples/
├── README.md          # This file
├── basic/
│   └── main.go        # Basic logging and tracing example
└── http_tracing/
    └── main.go        # HTTP trace propagation example
```

This structure allows you to:
- Run examples directly with `go run ./examples/[example_name]`
- Copy example directories as starting points for your own projects
- Understand different usage patterns of the observability library
