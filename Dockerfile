# Multi-stage Docker build for Cafnet Go API
# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile static Linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Final production image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/server /app/server

# Render injects $PORT at runtime, our server reads $PORT automatically
EXPOSE 8080

ENTRYPOINT ["/app/server"]
