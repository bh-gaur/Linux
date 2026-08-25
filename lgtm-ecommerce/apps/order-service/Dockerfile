# ==========================================
# STAGE 1: Build binary
# ==========================================
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Download dependencies
COPY order-service/go.mod order-service/go.sum* ./
RUN go mod download

# Copy source code
COPY order-service/main.go ./

# Build optimized static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o order-service main.go

# ==========================================
# STAGE 2: Minimal Execution Runner
# ==========================================
FROM alpine:3.18 AS runner

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/order-service ./

EXPOSE 8094

CMD ["./order-service"]
