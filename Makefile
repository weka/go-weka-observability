# =============================================================================
# DEPRECATED: Use Taskfile.yaml instead
# =============================================================================
# This Makefile is kept for backward compatibility but is deprecated.
# Please use the Taskfile.yaml with `task` command instead:
#
#   brew install go-task/tap/go-task  # Install task runner
#   task --list                       # List available tasks
#   task lintwithfix                  # Primary lint command (replaces make lint)
#   task test                         # Run tests
#   task                              # Run full quality pipeline
#
# The Taskfile provides:
#   - Local golangci-lint installation (no global install needed)
#   - Incremental linting (lint-new, lint-new-from-main)
#   - Better formatting with golangci-lint v2
#   - Coverage reports
# =============================================================================

.PHONY: build test lint vet run-example-logger-init run-example-basic run-example-http-tracing run-example-logger-simple run-example-logspan

tidy:
	go mod tidy -v

build:
	go build ./...

test: vet
	go test ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

# Run examples
run-example-logger-init:
	go run examples/logger_initialization.go

run-example-basic:
	go run examples/basic/main.go

run-example-http-tracing:
	go run examples/http_tracing/main.go

run-example-logger-simple:
	go run examples/test_logger_simple.go

run-example-logspan:
	go run examples/test_logspan.go