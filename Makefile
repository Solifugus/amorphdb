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

# Version metadata stamped into the binaries via -ldflags -X. VERSION can be
# overridden on the command line (e.g. `make build VERSION=1.2.3`); it defaults
# to the latest git tag, or "dev" when there are no tags.
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG=github.com/solifugus/amorphdb/internal/version
LDFLAGS=-X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).Date=$(BUILD_DATE)

all: build

## Build all binaries
build:
	@echo "Building AmorphDB binaries ($(VERSION))..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME_DAEMON) $(CMD_DIR)/$(BINARY_NAME_DAEMON)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME_CLIENT) $(CMD_DIR)/$(BINARY_NAME_CLIENT)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME_CONTROL) $(CMD_DIR)/$(BINARY_NAME_CONTROL)
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

## Install binaries to GOPATH/bin (development)
install: build
	@echo "Installing AmorphDB binaries to development environment..."
	cp $(BUILD_DIR)/$(BINARY_NAME_DAEMON) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(BINARY_NAME_CLIENT) $(GOPATH)/bin/
	cp $(BUILD_DIR)/$(BINARY_NAME_CONTROL) $(GOPATH)/bin/
	@echo "✅ Development install complete"

## System-wide installation with service setup (requires sudo)
install-system: build
	@echo "Installing AmorphDB system-wide..."
	@if [ "$(shell id -u)" != "0" ]; then \
		echo "❌ System installation requires sudo"; \
		echo "   Run: sudo make install-system"; \
		exit 1; \
	fi
	./scripts/install.sh
	@echo "✅ System installation complete"

## Uninstall system installation
uninstall-system:
	@echo "Uninstalling AmorphDB from system..."
	@if [ "$(shell id -u)" != "0" ]; then \
		echo "❌ System uninstall requires sudo"; \
		echo "   Run: sudo make uninstall-system"; \
		exit 1; \
	fi
	./scripts/install.sh uninstall
	@echo "✅ System uninstall complete"

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

## Build Docker image
docker-build:
	@echo "Building AmorphDB Docker image..."
	docker build -t amorphdb:latest .
	@echo "✅ Docker image built: amorphdb:latest"

## Start Docker Compose mesh
docker-up:
	@echo "Starting AmorphDB mesh with Docker Compose..."
	docker-compose up -d
	@echo "Waiting for nodes to start..."
	sleep 10
	docker-compose --profile setup run --rm amorphdb-setup
	@echo "✅ AmorphDB mesh is running"
	@echo "   Node 1: http://localhost:8080"
	@echo "   Node 2: http://localhost:8081"
	@echo "   Node 3: http://localhost:8082"

## Stop Docker Compose mesh
docker-down:
	@echo "Stopping AmorphDB mesh..."
	docker-compose down -v
	@echo "✅ AmorphDB mesh stopped"

## View Docker logs
docker-logs:
	docker-compose logs -f

## Docker development shell
docker-shell:
	docker run -it --rm amorphdb:latest sh

## Push Docker image (requires login)
docker-push:
	@echo "Pushing Docker image..."
	docker tag amorphdb:latest solifugus/amorphdb:latest
	docker push solifugus/amorphdb:latest
	@echo "✅ Docker image pushed"

## Build all release formats
release-all: clean
	@echo "Building complete release package..."
	./scripts/build-release.sh
	./scripts/build-packages.sh
	./scripts/build-docker.sh production
	@echo "✅ All release formats built"

## Build binary releases only
release-binaries:
	@echo "Building binary releases..."
	./scripts/build-release.sh
	@echo "✅ Binary releases complete"

## Build Linux packages only
release-packages:
	@echo "Building Linux packages..."
	./scripts/build-packages.sh
	@echo "✅ Package releases complete"

## Build Docker images only
release-docker:
	@echo "Building Docker images..."
	./scripts/build-docker.sh production
	@echo "✅ Docker releases complete"

## Make scripts executable
setup-release:
	@echo "Setting up release scripts..."
	chmod +x scripts/*.sh
	@echo "✅ Release scripts ready"

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
	@echo "  make build              # Build all binaries"
	@echo "  make test               # Run unit tests"
	@echo "  make install            # Install to development environment"
	@echo "  sudo make install-system # System-wide installation with service"
	@echo "  make docker-build       # Build Docker image"
	@echo "  make docker-up          # Start Docker mesh"
	@echo "  make release-all        # Build all release formats"
	@echo "  make dev                # Start development environment"