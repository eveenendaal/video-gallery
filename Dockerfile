FROM ubuntu:jammy

COPY views /app/views
COPY video-gallery /app/veenendaal-website

WORKDIR /app

CMD ["/app/video-gallery"]
