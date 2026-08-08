# Build stage - Go binary
FROM golang:1.23-alpine AS builder

ARG VERSION=dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.version=${VERSION} -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o analytics \
    ./cmd/analytics

# Build stage - Dashboard SPA
FROM node:22-alpine AS dashboard

WORKDIR /dashboard
COPY dashboard/package*.json ./
RUN npm ci
COPY dashboard/ .
RUN npm run build

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 analytics && \
    adduser -D -u 1000 -G analytics -h /var/lib/analytics -s /sbin/nologin analytics

RUN mkdir -p /var/lib/analytics /var/log/analytics /app/dashboard && \
    chown -R analytics:analytics /var/lib/analytics /var/log/analytics /app

WORKDIR /app

COPY --from=builder /build/analytics /app/analytics
COPY --from=builder /build/analytics.js /app/dashboard/analytics.js
COPY --from=dashboard /dashboard/dist/ /app/dashboard/

RUN chown -R analytics:analytics /app

USER analytics

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3001/api/dashboard/health || exit 1

ENV ANALYTICS_ADDR=:3001 \
    ANALYTICS_DB_PATH=/var/lib/analytics/analytics.db \
    ANALYTICS_RETENTION_DAYS=90

ENTRYPOINT ["/app/analytics"]
