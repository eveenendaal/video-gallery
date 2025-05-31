FROM rust:alpine as builder

WORKDIR /app
COPY . .
RUN apk add --no-cache musl-dev openssl-dev pkgconfig && \
    cargo build --release

FROM alpine:latest
WORKDIR /app
COPY --from=builder --chmod=755 /app/target/release/video-gallery /app/video-gallery
COPY --chmod=644 public /app/public
COPY --chmod=644 templates /app/templates
CMD ["/app/video-gallery"]
