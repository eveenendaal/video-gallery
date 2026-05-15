# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this repository.

## Commands

### Go (backend)
```bash
go build ./...                   # compile
go run main.go serve             # run locally (requires env vars below)
go test ./...                    # run all tests
go test ./internal/application/  # run a single package's tests
go vet ./...                     # lint
```

### CSS (frontend)
```bash
npm install      # install sass + bulma
npm run build    # compile assets/scss/styles.scss → public/styles.css
```

### Docker
```bash
docker build -f build/Dockerfile -t video-gallery .
docker run -p 8080:8080 \
  -e SECRET_KEY=mykey \
  -e BUCKET_NAME=mybucket \
  -e TMDB_API_KEY=optional \
  -v ~/.config/gcloud:/home/appuser/.config/gcloud:ro \
  video-gallery
```

## Required Environment Variables

| Variable | Required | Description |
|---|---|---|
| `SECRET_KEY` | yes | Prefix for all protected routes (`/{SECRET_KEY}/index`, etc.) |
| `BUCKET_NAME` | yes | GCS bucket containing video files |
| `TMDB_API_KEY` | no | TMDb API key for movie poster fetching |
| `PORT` | no | HTTP listen port (default `8080`) |

## Architecture

The project uses a clean layered architecture:

```
internal/domain/gallery/     ← domain entities + interface contracts (no deps)
internal/application/        ← business logic services (depend on domain interfaces)
internal/infrastructure/     ← concrete implementations (GCS, FFmpeg, TMDb)
pkg/handlers/                ← HTTP handlers (depend on application services)
pkg/config/                  ← env-var loading
cmd/                         ← Cobra CLI wiring
assets/templates/            ← Pug HTML templates (compiled at request time)
assets/scss/                 ← SCSS source (compiled to public/styles.css at build time)
```

**Dependency wiring** happens entirely in `cmd/serve_cmd.go:serveWebsite()`. Infrastructure implementations are constructed there and injected into application services, which are injected into handlers.

**Domain interfaces** (`internal/domain/gallery/repository.go`) define the contracts that infrastructure must satisfy:
- `StorageRepository` — list, sign, upload, download, delete objects in GCS
- `VideoProcessor` — extract video frames and validate images (FFmpeg)
- `MoviePosterClient` — search TMDb and download poster images

## Key Behaviours

**Bucket structure** — The app expects objects at exactly three path segments: `Category/Group/Filename.ext`. Files at any other depth are silently ignored. Video and thumbnail files share the same base name; e.g. `Movies/Action/Terminator.mp4` pairs with `Movies/Action/Terminator.jpg`.

**Gallery stubs** — Each gallery's URL is a 4-character base64 prefix of SHA-256(`galleryName + secretKey`). This lets individual galleries be shared without exposing the full index path.

**Caching** — `GalleryService` caches the full video list in-process for 5 minutes. Any write operation (thumbnail generate/clear/upload) calls `InvalidateCache()` immediately after so the next read fetches fresh state.

**SSE progress streaming** — `GenerateThumbnail` and `FetchMoviePoster` support both POST (JSON response) and GET (EventSource / SSE) on the same endpoint. The GET form is used by the admin UI to stream real-time progress events.

**Thumbnail generation** — Downloads the video to `os.TempDir()/video-gallery-thumbnails/`, runs `ffmpeg -ss <time> -i <video> -vf thumbnail -frames:v 1`, validates the output is not a solid-colour frame (≥1% of sampled pixels must differ), then uploads the result back to GCS.

**Pug templates** — Templates in `assets/templates/` are compiled on every request via `pug.CompileFile`. There is no pre-compilation step; changes to `.pug` files take effect immediately without restarting the server.

**SCSS / CSS** — `npm run build` must be re-run after any change to `assets/scss/`. The compiled output (`public/styles.css`) is committed to the repo and served as a static file. Bulma is the CSS framework.

## Route Map

All user-visible routes are registered in `cmd/serve_cmd.go`. The static file server is mounted at `/` and serves `./public/`.

| Path | Handler |
|---|---|
| `/{key}/index` | Gallery index page (Pug) |
| `/{key}/feed` | JSON feed for tvOS player |
| `/gallery/{stub}` | Individual gallery page (Pug) |
| `/{key}/admin` | Admin UI (Pug) |
| `/{key}/admin/api/generate-thumbnail` | GET (SSE) or POST |
| `/{key}/admin/api/clear-thumbnail` | POST |
| `/{key}/admin/api/bulk-generate-thumbnails` | POST |
| `/{key}/admin/api/bulk-clear-thumbnails` | POST |
| `/{key}/admin/api/fetch-movie-poster` | GET (SSE) or POST |
| `/{key}/admin/api/search-movie-poster` | GET |
