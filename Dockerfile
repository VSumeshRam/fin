# Stage 1: Build the optimized Go Gateway release binary
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Build static binary without CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/gateway ./cmd/gateway/main.go

# Stage 2: Production Minimal Runtime
FROM alpine:3.20

# Install TLS certificates for secure HTTPS egress to OpenAI / Anthropic
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/gateway /app/gateway

# Copy required static assets & RBAC configurations
COPY rbac/ /app/rbac/
COPY internal/dashboard/static/ /app/internal/dashboard/static/

# Default environment configuration
ENV PORT=8080 \
    DASHBOARD_PORT=3000 \
    REDIS_ADDR=redis:6379 \
    ML_SIDECAR_URL=http://ml-sidecar:8000 \
    UPSTREAM_URL=https://api.openai.com

# Expose Gateway API (8080) and Executive Dashboard (3000)
EXPOSE 8080 3000

CMD ["/app/gateway"]
