FROM ubuntu:jammy

# Install the CAs
RUN apt-get update && apt-get install -y ca-certificates

# Install App
COPY views /app/views
COPY public /app/public
COPY video-gallery /app/video-gallery
RUN chmod +x /app/video-gallery

WORKDIR /app

CMD ["/app/video-gallery"]
