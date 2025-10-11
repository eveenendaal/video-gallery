# Build stage
FROM golang:1.25-alpine3.22 AS builder

WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build argument for version
ARG VERSION=dev

# Build the application with version injection and optimization flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X video-gallery/cmd.Version=${VERSION}" \
    -trimpath \
    -o video-gallery

# Final stage
FROM alpine:3.22

# Install CA certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Create non-root user for running the application
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Create app directory
WORKDIR /app

# Copy built binary from builder stage with executable permissions
COPY --from=builder --chmod=755 /build/video-gallery /app/video-gallery

# Copy required application files with appropriate permissions
COPY --from=builder --chmod=644 /build/views /app/views
COPY --from=builder --chmod=644 /build/public /app/public

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port (documentation only, actual port configured via environment)
EXPOSE 8080

# Run the application
CMD ["/app/video-gallery", "serve"]
