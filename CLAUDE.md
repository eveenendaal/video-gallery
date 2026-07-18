# Video Gallery

Go web server that browses/streams a personal video library stored in a single
cloud storage bucket, organized as `Category/Gallery/Video.ext` (+ optional
same-basename `.jpg`/`.png` thumbnail). Stdlib `net/http` + Cobra CLI + Pug
templates. No database — the bucket listing is the source of truth, cached
in-memory for 5 minutes.

## Commands

```bash
make build            # go build ./...
make frontend-build    # npm install + npm run build (SCSS -> public/styles.css)
go test ./...
```

## Architecture

DDD-ish layout, seamed around `internal/domain/gallery/repository.go`'s
`StorageRepository` interface:

- `internal/domain/gallery/` — entities (`Video`, `Gallery`, `Category`) and interface contracts (`StorageRepository`, `VideoProcessor`, `MoviePosterClient`)
- `internal/application/` — use-case services (`GalleryService`, `ThumbnailService`, `PosterService`)
- `internal/infrastructure/gcs/`, `internal/infrastructure/r2/` — the two `StorageRepository` implementations (see Storage Backends below)
- `internal/infrastructure/ffmpeg/`, `internal/infrastructure/tmdb/` — thumbnail extraction, movie poster lookup
- `pkg/handlers/` — HTTP handlers; `pkg/config/` — env-var config loader
- `cmd/serve_cmd.go` — wires everything together, including backend selection

## Storage Backends

Selected via `STORAGE_BACKEND` (`gcs`, the default, or `r2`), both implementing
the same `gallery.StorageRepository` interface so the rest of the app is
backend-agnostic. Adding a third backend means implementing that interface and
adding a case in `cmd/serve_cmd.go`'s `newStorageRepository`.

- `gcs` — Google Cloud Storage, auth via Application Default Credentials. Needs only `BUCKET_NAME`.
- `r2` — Cloudflare R2 via its S3-compatible API, auth via static credentials. Needs `BUCKET_NAME`, `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`.

Both backends serve media via time-limited presigned/signed GET URLs generated
per-request (never public buckets) — the browser fetches media directly from
the bucket, bypassing this app entirely.

## Deployment

This repo is **public** and does not deploy itself — no deploy step in its own
GitHub Actions. `.github/workflows/deploy.yml` only builds and pushes a
version-tagged image to `ghcr.io/eveenendaal/video-gallery`. Actual deployment
(to Cloudflare Workers+Containers) is orchestrated entirely from the sibling
`terraform-base` repo, which polls this repo's release tags and owns all
deploy credentials — see that repo's `CLAUDE.md` and
`.github/workflows/deploy-video-gallery.yml`. The `terraform/` directory here
is a stale example copy, not the real infrastructure.

## Environment Variables

See `README.md` for the full list (`SECRET_KEY`, `BUCKET_NAME`, `PORT`,
`TMDB_API_KEY`, `STORAGE_BACKEND` + R2-specific vars).
