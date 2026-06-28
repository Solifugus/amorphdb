# AmorphDB Docker Image
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN make build

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create amorphdb user
RUN addgroup -g 1001 amorphdb && \
    adduser -D -s /bin/sh -u 1001 -G amorphdb amorphdb

# Create directories
RUN mkdir -p /var/lib/amorphdb /var/log/amorphdb /etc/amorphdb && \
    chown -R amorphdb:amorphdb /var/lib/amorphdb /var/log/amorphdb /etc/amorphdb

# Copy binaries from builder
COPY --from=builder /app/bin/amorphd /usr/local/bin/
COPY --from=builder /app/bin/amorph /usr/local/bin/
COPY --from=builder /app/bin/amorphctl /usr/local/bin/

# Copy configuration
COPY --from=builder /app/examples/mesh_setup/configs/basic_config.yaml /etc/amorphdb/amorphd.yaml

# Fix ownership
RUN chown amorphdb:amorphdb /etc/amorphdb/amorphd.yaml

# Switch to amorphdb user
USER amorphdb

# Expose ports
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD amorphctl status || exit 1

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/amorphd"]
CMD ["--config=/etc/amorphdb/amorphd.yaml"]