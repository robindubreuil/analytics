# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build \
    -ldflags "-X main.version=${VERSION} -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o analytics \
    ./cmd/analytics

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 analytics && \
    adduser -D -u 1000 -G analytics -h /var/lib/analytics -s /sbin/nologin analytics

# Create directories
RUN mkdir -p /var/lib/analytics /var/log/analytics && \
    chown -R analytics:analytics /var/lib/analytics /var/log/analytics

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/analytics /app/analytics

# Set ownership
RUN chown analytics:analytics /app/analytics

# Switch to non-root user
USER analytics

# Expose port
EXPOSE 3001

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3001/api/dashboard/health || exit 1

# Set environment variables
ENV ANALYTICS_ADDR=:3001 \
    ANALYTICS_DB_PATH=/var/lib/analytics/analytics.db \
    ANALYTICS_RETENTION_DAYS=90

# Run the binary
ENTRYPOINT ["/app/analytics"]
