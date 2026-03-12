# AmorphDB Makefile

.PHONY: all build test clean install docs help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build targets
BINARY_NAME_DAEMON=amorphd
BINARY_NAME_CLIENT=amorph
BINARY_NAME_CONTROL=amorphctl

# Directories
BUILD_DIR=./bin
CMD_DIR=./cmd

all: build

## Build all binaries
build:
	@echo "Building AmorphDB binaries..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME_DAEMON) $(CMD_DIR)/$(BINARY_NAME_DAEMON)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME_CLIENT) $(CMD_DIR)/$(BINARY_NAME_CLIENT)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME_CONTROL) $(CMD_DIR)/$(BINARY_NAME_CONTROL)
	@echo "✅ Build complete: binaries in $(BUILD_DIR)/"

## Run unit tests
test:
	@echo "Running unit tests..."
	$(GOTEST) -v ./...
	@echo "✅ Tests complete"

## Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v ./test/integration/...
	@echo "✅ Integration tests complete"

## Run all tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

## Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

## Install binaries to GOPATH/bin
install: build
	@echo "Installing AmorphDB binaries..."
	cp $(BUILD_DIR)/$(BINARY_NAME_DAEMON) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(BINARY_NAME_CLIENT) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(BINARY_NAME_CONTROL) $(GOPATH)/bin/
	@echo "✅ Install complete"

## Update Go modules
mod-tidy:
	$(GOMOD) tidy
	@echo "✅ Go modules updated"

## Format Go code
fmt:
	@echo "Formatting Go code..."
	$(GOCMD) fmt ./...
	@echo "✅ Code formatting complete"

## Run Go linter
lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin; }
	golangci-lint run
	@echo "✅ Linting complete"

## Clean up project structure
cleanup:
	@echo "Cleaning up project structure..."
	python3 scripts/cleanup_project.py
	@echo "✅ Project cleanup complete"

## Generate documentation
docs:
	@echo "Generating documentation..."
	@command -v godoc >/dev/null 2>&1 || { echo "Installing godoc..."; $(GOGET) golang.org/x/tools/cmd/godoc; }
	@echo "📖 Documentation available at: http://localhost:6060/pkg/github.com/yourusername/amorphdb/"
	@echo "💡 Run: godoc -http=:6060"

## Start development environment
dev: build
	@echo "Starting AmorphDB development environment..."
	./$(BUILD_DIR)/$(BINARY_NAME_DAEMON) &
	sleep 2
	./$(BUILD_DIR)/$(BINARY_NAME_CLIENT)

## Build Docker image (if Dockerfile exists)
docker:
	@if [ -f Dockerfile ]; then \
		echo "Building Docker image..."; \
		docker build -t amorphdb .; \
		echo "✅ Docker image built: amorphdb"; \
	else \
		echo "❌ Dockerfile not found"; \
	fi

## Show this help message
help:
	@echo "AmorphDB Build System"
	@echo ""
	@echo "Available targets:"
	@awk '/^##/ { \
		split($$0, a, "##"); \
		target = ""; \
		getline; \
		if ($$0 ~ /^[a-zA-Z-]+:/) { \
			split($$0, b, ":"); \
			target = b[1]; \
		} \
		if (target != "") \
			printf "  %-20s %s\n", target, a[2]; \
	}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Examples:"
	@echo "  make build          # Build all binaries"
	@echo "  make test           # Run unit tests"
	@echo "  make cleanup        # Organize project structure"
	@echo "  make dev            # Start development environment"