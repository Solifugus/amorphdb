#!/bin/bash
# AmorphDB Docker Image Builder
# Creates production Docker images with pre-built binaries

set -euo pipefail

# Configuration
VERSION="${VERSION:-$(date +%Y.%m.%d)}"
REGISTRY="${REGISTRY:-amorphdb}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

# Check dependencies
check_deps() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker is required but not installed"
        exit 1
    fi
}

# Build binaries for Linux platforms
build_binaries() {
    log_info "Building binaries for Docker..."

    # Build for amd64
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/linux-amd64/amorphd ./cmd/amorphd
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/linux-amd64/amorph ./cmd/amorph
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/linux-amd64/amorphctl ./cmd/amorphctl

    # Build for arm64
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/linux-arm64/amorphd ./cmd/amorphd
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/linux-arm64/amorph ./cmd/amorph
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/linux-arm64/amorphctl ./cmd/amorphctl

    log_success "Binaries built for Linux platforms"
}

# Create production Dockerfile (binary-only)
create_production_dockerfile() {
    cat > Dockerfile.production << 'EOF'
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

# Create amorphdb user
RUN addgroup -g 1001 amorphdb && \
    adduser -D -s /bin/sh -u 1001 -G amorphdb amorphdb

# Create directories
RUN mkdir -p /var/lib/amorphdb /var/log/amorphdb /etc/amorphdb && \
    chown -R amorphdb:amorphdb /var/lib/amorphdb /var/log/amorphdb /etc/amorphdb

# Copy architecture-specific binaries
ARG TARGETARCH
COPY bin/linux-${TARGETARCH}/* /usr/local/bin/

# Copy configuration and documentation
COPY examples/mesh_setup/configs/basic_config.yaml /etc/amorphdb/amorphd.yaml
COPY docs/ /usr/share/doc/amorphdb/
COPY README.md INSTALL.md /usr/share/doc/amorphdb/

# Fix ownership
RUN chown amorphdb:amorphdb /etc/amorphdb/amorphd.yaml

# Switch to amorphdb user
USER amorphdb

# Set working directory
WORKDIR /var/lib/amorphdb

# Expose ports
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD amorphctl status || exit 1

# Labels
LABEL org.opencontainers.image.title="AmorphDB" \
      org.opencontainers.image.description="AmorphDB Temporal Database with Multi-Mesh Architecture" \
      org.opencontainers.image.vendor="AmorphDB" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.schema-version="1.0"

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/amorphd"]
CMD ["--config=/etc/amorphdb/amorphd.yaml"]
EOF
}

# Build multi-arch Docker image
build_docker_image() {
    log_info "Building multi-architecture Docker image..."

    # Create buildx builder if it doesn't exist
    if ! docker buildx inspect amorphdb-builder >/dev/null 2>&1; then
        docker buildx create --name amorphdb-builder --use
    fi

    # Build and push multi-arch image
    docker buildx build \
        --platform "$PLATFORMS" \
        --tag "$REGISTRY/amorphdb:$VERSION" \
        --tag "$REGISTRY/amorphdb:latest" \
        --file Dockerfile.production \
        --push \
        .

    log_success "Docker image built and pushed: $REGISTRY/amorphdb:$VERSION"
}

# Create Docker Compose for distribution
create_production_compose() {
    cat > docker-compose.production.yml << EOF
# AmorphDB Production Docker Compose
version: '3.8'

services:
  amorphdb:
    image: $REGISTRY/amorphdb:$VERSION
    container_name: amorphdb
    ports:
      - "8080:8080"
    volumes:
      - amorphdb_data:/var/lib/amorphdb
      - amorphdb_logs:/var/log/amorphdb
      - ./amorphd.yaml:/etc/amorphdb/amorphd.yaml:ro
    environment:
      - AMORPHDB_NODE_ID=\${HOSTNAME}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "amorphctl", "status"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  amorphdb_data:
  amorphdb_logs:
EOF

    # Create example configuration
    cat > amorphd-example.yaml << 'EOF'
# AmorphDB Production Configuration
data:
  storage_dir: "/var/lib/amorphdb"

network:
  port: 8080
  local_socket_path: "/var/run/amorphdb.sock"
  max_connections: 1000
  bind_address: "0.0.0.0"

mesh:
  name: ""                    # Set your mesh name
  identity: "${HOSTNAME}"     # Node identifier

logging:
  level: "info"
  file: "/var/log/amorphdb/amorphdb.log"
  max_size: "100MB"
  max_backups: 10
  compress: true

security:
  enable_auth: true           # Enable for production
EOF

    log_success "Created production Docker Compose configuration"
}

# Create minimal Dockerfile for local builds (no registry push)
create_local_dockerfile() {
    cat > Dockerfile.local << 'EOF'
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1001 amorphdb && \
    adduser -D -s /bin/sh -u 1001 -G amorphdb amorphdb && \
    mkdir -p /var/lib/amorphdb /var/log/amorphdb /etc/amorphdb && \
    chown -R amorphdb:amorphdb /var/lib/amorphdb /var/log/amorphdb /etc/amorphdb

COPY bin/amorphd bin/amorph bin/amorphctl /usr/local/bin/
COPY examples/mesh_setup/configs/basic_config.yaml /etc/amorphdb/amorphd.yaml

RUN chown amorphdb:amorphdb /etc/amorphdb/amorphd.yaml

USER amorphdb
WORKDIR /var/lib/amorphdb
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD amorphctl status || exit 1

ENTRYPOINT ["/usr/local/bin/amorphd"]
CMD ["--config=/etc/amorphdb/amorphd.yaml"]
EOF
}

# Main execution
main() {
    case "${1:-production}" in
        production)
            log_info "Building production Docker images for $REGISTRY/amorphdb:$VERSION"
            check_deps
            build_binaries
            create_production_dockerfile
            build_docker_image
            create_production_compose
            ;;

        local)
            log_info "Building local Docker image"
            check_deps
            make build  # Use local binaries
            create_local_dockerfile
            docker build -t amorphdb:local -f Dockerfile.local .
            log_success "Local Docker image built: amorphdb:local"
            ;;

        --help|-h)
            echo "AmorphDB Docker Builder"
            echo ""
            echo "Usage: $0 [production|local|--help]"
            echo ""
            echo "Commands:"
            echo "  production  Build and push multi-arch production images"
            echo "  local       Build local development image"
            echo ""
            echo "Environment variables:"
            echo "  VERSION     Version tag (default: current date)"
            echo "  REGISTRY    Docker registry (default: amorphdb)"
            echo "  PLATFORMS   Target platforms (default: linux/amd64,linux/arm64)"
            echo ""
            echo "Examples:"
            echo "  VERSION=1.0.0 REGISTRY=myregistry $0 production"
            echo "  $0 local"
            ;;

        *)
            echo "Unknown command: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
}

main "$@"