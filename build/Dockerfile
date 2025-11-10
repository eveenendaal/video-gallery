# Multi-stage Dockerfile for Video Gallery application
# This builds a containerized video streaming and gallery application
# with a Go backend and compiled frontend assets.

# Frontend build stage
FROM node:24-alpine AS frontend-builder

WORKDIR /frontend

# Copy package files first for better caching
COPY package*.json ./
RUN npm ci --omit=dev

# Copy source files needed for build
COPY assets/scss ./assets/scss

# Build frontend (compile SCSS to CSS)
RUN npm run build

# Backend build stage
FROM golang:1.25-alpine3.22 AS builder

# Install CA certificates for downloading Go modules
RUN apk add --no-cache ca-certificates

WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Copy built frontend assets from frontend-builder
COPY --from=frontend-builder /frontend/public ./public

# Build argument for version
ARG VERSION=dev

# Build the application with version injection and optimization flags
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X video-gallery/cmd.Version=${VERSION}" \
    -trimpath \
    -o video-gallery

# Final stage
FROM alpine:3.22

# OCI labels
LABEL org.opencontainers.image.title="video-gallery"
LABEL org.opencontainers.image.description="Video gallery and streaming application"
LABEL org.opencontainers.image.authors="Eric Veenendaal <eric@ericveenendaal.com>"
LABEL org.opencontainers.image.source="https://github.com/eveenendaal/video-gallery"
LABEL org.opencontainers.image.licenses="GPL-3.0"
LABEL org.opencontainers.image.documentation="https://github.com/eveenendaal/video-gallery/blob/master/README.md"

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
COPY --from=builder /build/assets /app/assets
COPY --from=builder /build/public /app/public

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port (documentation only, actual port configured via environment)
EXPOSE 8080

# Run the application
CMD ["/app/video-gallery", "serve"]
