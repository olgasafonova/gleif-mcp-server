# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gleif-mcp-server .

# Final stage - minimal image
FROM alpine:3.20

# Add ca-certificates for HTTPS requests to GLEIF API
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/gleif-mcp-server /app/gleif-mcp-server

# MCP Registry label for OCI discovery
LABEL io.modelcontextprotocol.server.name="io.github.olgasafonova/gleif-mcp-server"

# Run as non-root user
RUN adduser -D -u 1000 mcp
USER mcp

ENTRYPOINT ["/app/gleif-mcp-server"]
