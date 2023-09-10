FROM ubuntu:jammy

COPY views /app/views
COPY public /app/public
COPY video-gallery /app/veenendaal-website

WORKDIR /app

CMD ["/app/video-gallery"]
