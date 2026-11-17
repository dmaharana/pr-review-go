BINARY_NAME=pr-review-go
BUILD_DIR=bin
CMD_PATH=./cmd/pr-review-go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_TIME)

.PHONY: all build test test-cover lint clean run tidy help

all: build

## build: Build the single binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

## test: Run unit tests
test:
	go test -v ./...

## test-cover: Run unit tests with code coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## lint: Run go vet
lint:
	go vet ./...

## tidy: Tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## clean: Remove build artifacts and temporary test output
clean:
	rm -rf $(BUILD_DIR) coverage.out pr-review-go

## run: Build and run the server daemon on port 8080
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) serve --port=8080

## help: Display this help message
help:
	@echo "Available make targets for $(BINARY_NAME):"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
