FROM rust:alpine AS builder

WORKDIR /app
COPY . .

# Install build dependencies and build the application
RUN apk add --no-cache musl-dev openssl-dev pkgconfig build-base openssl-libs-static && \
    cargo build --release

FROM alpine:latest

# Add runtime dependencies
RUN apk add --no-cache ca-certificates libgcc libssl3

WORKDIR /app

# Copy binary with executable permissions
COPY --chmod=755 --from=builder /app/target/release/video-gallery /app/video-gallery

# Copy static assets with read-only permissions
COPY --chmod=644 public /app/public
COPY --chmod=644 templates /app/templates

# Set default port - can be overridden at runtime
ARG PORT=8080
ENV PORT=${PORT}
EXPOSE ${PORT}
ENV RUST_LOG=info

# Run as non-root user for better security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app
USER appuser

CMD ["/app/video-gallery"]
