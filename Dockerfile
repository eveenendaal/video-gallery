# Multi-stage Dockerfile for Video Gallery application (Rust)
# This builds a containerized video streaming and gallery application
# with a Rust backend and compiled frontend assets.

# Backend build stage
FROM rust:1.83-alpine AS builder

# Install build dependencies
RUN apk add --no-cache musl-dev pkgconfig openssl-dev

WORKDIR /build

# Copy manifests first for better caching
COPY Cargo.toml ./

# Create a dummy src/main.rs to cache dependencies
RUN mkdir -p src && \
    echo "fn main() {}" > src/main.rs && \
    cargo build --release && \
    rm -rf src

# Copy the actual source code
COPY src ./src

# Build the application
ARG VERSION=dev
RUN touch src/main.rs && \
    cargo build --release

# Final stage
FROM alpine:3.22

# OCI labels
LABEL org.opencontainers.image.title="video-gallery"
LABEL org.opencontainers.image.description="Video gallery and streaming application"
LABEL org.opencontainers.image.authors="Eric Veenendaal <eric@ericveenendaal.com>"
LABEL org.opencontainers.image.source="https://github.com/eveenendaal/video-gallery"
LABEL org.opencontainers.image.licenses="GPL-3.0"
LABEL org.opencontainers.image.documentation="https://github.com/eveenendaal/video-gallery/blob/master/README.md"

# Install CA certificates for HTTPS requests and ffmpeg for thumbnail generation
RUN apk add --no-cache ca-certificates ffmpeg

# Create non-root user for running the application
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Create app directory
WORKDIR /app

# Copy built binary from builder stage with executable permissions
COPY --from=builder --chmod=755 /build/target/release/video-gallery /app/video-gallery

# Copy required application files with appropriate permissions
# Note: public/styles.css should be built locally with `npm run build` before building the Docker image
COPY assets /app/assets
COPY public /app/public

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port (documentation only, actual port configured via environment)
EXPOSE 8080

# Run the application
CMD ["/app/video-gallery"]
