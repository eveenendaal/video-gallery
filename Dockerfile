FROM ubuntu:jammy

COPY views /app/views
COPY public /app/public
COPY video-gallery /app/video-gallery
RUN chmod +x /app/video-gallery

WORKDIR /app

CMD ["/app/video-gallery"]
