.PHONY: build test lint vet run-example-logger-init run-example-basic run-example-http-tracing run-example-logger-simple run-example-logspan

build:
	go build ./...

test:
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