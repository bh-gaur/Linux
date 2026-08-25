# ==========================================
# STAGE 1: Build binary
# ==========================================
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Download dependencies
COPY catalog-service/go.mod catalog-service/go.sum* ./
RUN go mod download

# Copy source code
COPY catalog-service/main.go ./

# Build optimized static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o catalog-service main.go

# ==========================================
# STAGE 2: Minimal Execution Runner
# ==========================================
FROM alpine:3.18 AS runner

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/catalog-service ./

EXPOSE 8092

CMD ["./catalog-service"]
