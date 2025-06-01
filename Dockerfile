# Build stage
FROM golang:alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o video-gallery ./bin/serve

# Final stage
FROM alpine:latest

# Install CA certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Create app directory
WORKDIR /app

# Copy built binary from builder stage with executable permissions
COPY --from=builder --chmod=755 /build/video-gallery /app/video-gallery

# Copy required application files with appropriate permissions
COPY --from=builder --chmod=644 /build/views /app/views
COPY --from=builder --chmod=644 /build/public /app/public

# Run the application
CMD ["/app/video-gallery"]
