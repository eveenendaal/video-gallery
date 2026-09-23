# Video Gallery

The goal of this project is to build a serverless ready application for displaying a users video content library using only a single storage bucket.

## Overview

Video Gallery is a web-based application that runs as a Docker container, providing an interface to browse and play videos organized in galleries. The application is designed to run on serverless platforms like Google Cloud Run, requiring only a single storage bucket for video files.

## Web Interface

The interface is pretty simple.

For the HTML index use:
```
GET /{SECRET_KEY}/index
```

For the Video Feed use:
```
GET /{SECRET_KEY}/feed
```

For the Admin interface use:
```
GET /{SECRET_KEY}/admin
```

**Security Note:** All admin endpoints (page and API) are protected by the secret key in the URL path. The admin API endpoints follow the pattern `/{SECRET_KEY}/admin/api/*`, ensuring that only users with knowledge of the secret key can perform administrative operations.

You can navigate to all the galleries from the HTML index page.  After clicking into one of these galleries, the application open a new page specifically for that gallery. Each gallery is given its own unique prefix. This means you'll be able to share an individual gallery with someone without revealing the path to all the galleries.

### Admin Interface

The admin interface provides powerful tools for managing video thumbnails:

**Features:**
- View all videos with their current thumbnail status
- Generate thumbnails from video frames with customizable time offset (for home movies)
- Fetch movie posters from TMDb database (for actual movies)
- Clear thumbnails for individual videos
- Filter and sort videos by category, gallery, or status
- Select multiple videos for bulk operations

**Individual Video Operations:**
Each video has controls to:
1. Choose between "From Video" (extract frame) or "Movie Poster" (fetch from TMDb)
2. For "From Video": Set the time in milliseconds where the thumbnail should be extracted (e.g., 1000ms = 1 second into the video)
3. For "Movie Poster": The system automatically extracts the movie title from the filename and searches TMDb
4. Generate/fetch the thumbnail
5. Clear the existing thumbnail (if present)

**Bulk Operations:**
1. Select multiple videos using checkboxes
2. Set a default time offset for frame extraction
3. Click "Generate Selected" to create thumbnails for selected videos
4. Click "Clear Selected" to remove thumbnails from selected videos
5. Adjust parallel operations (1-10) to control processing speed

Generated thumbnails and posters are always written as `<video name>.jpg` next to the video, replacing any existing `.jpg` thumbnail only once the new one has been produced successfully.

**Movie Poster Feature:**
When using "Movie Poster" mode, the system:
- Uses the video's name as the search title, dropping anything from the first `(` or `[` onward (e.g. `Alien (1979) [4K]` searches for `Alien`)
- Searches The Movie Database (TMDb) and lets you pick the matching movie
- Downloads the poster and stores it as the video's `.jpg` thumbnail
- Requires the `TMDB_API_KEY` environment variable to be set

The interface will show real-time progress of each operation and automatically refresh after successful completion.

## Feed Schema

`GET /{SECRET_KEY}/feed` returns a JSON array of galleries. The formal JSON Schema lives in [`schemas/video-gallery-feed-schema.json`](schemas/video-gallery-feed-schema.json). Video `url` and `thumbnail` values are signed URLs valid for 24 hours; `thumbnail` is omitted when a video has none.

### Feed Example

This is an example video feed. The URL and thumbnail values are just placeholders.

```json
[
    {
        "name": "Gallery 1",
        "category": "Category 1",
        "videos": [
            {
                "name": "Demo Video 1",
                "url": "https://domain.tld/video-1.mp4",
                "thumbnail": "https://domain.tld/example.jpg"
            }
        ]
    },
    {
        "name": "Gallery 2",
        "category": "Category 2",
        "videos": [
            {
                "name": "Demo Video 2",
                "url": "https://domain.tld/video-2.mp4"
            }
        ]
    }
]
```

### Integrations

#### [Video Feed Player](https://www.ericveenendaal.com/blog/video-feed-player)
This tvOS application is compatible with this video feed

## Code Structure

```
.
├── main.go                       # Entry point
├── cmd/                          # Cobra CLI; serve_cmd.go wires dependencies and routes
├── internal/
│   ├── domain/gallery/           # Entities, bucket layout rules, and interfaces
│   ├── application/              # Use cases: gallery listing/caching, thumbnails, posters
│   └── infrastructure/
│       ├── gcs/, r2/             # StorageRepository implementations
│       ├── ffmpeg/               # Frame extraction and blank-frame detection
│       └── tmdb/                 # Movie poster lookup
├── pkg/
│   ├── config/                   # Environment-variable configuration
│   └── handlers/                 # HTTP handlers
├── assets/                       # Pug templates and SCSS sources
├── public/                       # Static files and compiled CSS
├── schemas/                      # Feed JSON Schema
├── build/                        # Dockerfile
└── terraform/                    # Example infrastructure
```

## Development

Requires Go (see `go.mod`), Node.js (for the stylesheet), and `ffmpeg` on the `PATH` for thumbnail generation.

```bash
make frontend-build   # compile SCSS to public/styles.css
make test             # go test ./...
SECRET_KEY=dev BUCKET_NAME=your-bucket go run . serve
```

The server must run from the repository root, since it loads `assets/templates` and `public/` relative to the working directory. Command-line flags (`--secret-key`, `--bucket`, `--port`, `--storage-backend`) override the matching environment variables.

## Deployment

The application is a single container that needs only a storage bucket, so it runs well on serverless container platforms (e.g. Google Cloud Run or Cloudflare Containers) for essentially no cost.

### Container Image

The application is available as a Docker image at `ghcr.io/eveenendaal/video-gallery`. Available tags include:
- `latest` - Most recent build from the master branch
- Version tags (e.g., `2.0.171`) - Specific releases
- Major.minor tags (e.g., `2.0`) - Latest patch version in that series
- Major version tags (e.g., `2`) - Latest minor version in that series

To deploy:
1. Run the image on your container platform of choice
2. Give it credentials for your bucket (see [Storage Backends](#storage-backends)); the admin features need write access for thumbnails
3. Set the following environment variables:

**BUCKET_NAME** - The bucket with the video files. This is needed to access the bucket.

**SECRET_KEY** - A long, random string. It prefixes the index, feed, and admin URLs and salts the per-gallery URLs, so treat it like a password.

**PORT** (Optional) - Port to listen on. Defaults to `8080`.

**TMDB_API_KEY** (Optional) - API key from The Movie Database (TMDb) for fetching movie posters. Required if you want to use the "Movie Poster" feature in the admin panel. Get a free API key at https://www.themoviedb.org/settings/api

**STORAGE_BACKEND** (Optional) - Which storage backend to use: `gcs` (default) or `r2`. See [Storage Backends](#storage-backends) below.

### Storage Backends

The application supports two interchangeable storage backends behind the same `BUCKET_NAME`/gallery-layout convention. Select one with `STORAGE_BACKEND`:

#### Google Cloud Storage (`STORAGE_BACKEND=gcs`, default)

Uses Application Default Credentials — no extra environment variables needed beyond `BUCKET_NAME`. Locally, run `gcloud auth login --update-adc` and mount `~/.config/gcloud` into the container.

#### Cloudflare R2 (`STORAGE_BACKEND=r2`)

Uses R2's S3-compatible API with static credentials. Requires:

**R2_ACCOUNT_ID** - Your Cloudflare account ID.

**R2_ACCESS_KEY_ID** / **R2_SECRET_ACCESS_KEY** - S3-compatible credentials from an R2 API token (Object Read & Write, scoped to the bucket).

```bash
docker run -p 8080:8080 \
  -e SECRET_KEY=your-secret-key \
  -e BUCKET_NAME=your-bucket-name \
  -e STORAGE_BACKEND=r2 \
  -e R2_ACCOUNT_ID=your-cloudflare-account-id \
  -e R2_ACCESS_KEY_ID=your-r2-access-key-id \
  -e R2_SECRET_ACCESS_KEY=your-r2-secret-access-key \
  ghcr.io/eveenendaal/video-gallery:latest
```

#### Terraform

You can find example terraform code in the [terraform](terraform) directory.

### Running Locally

To run the application locally using Docker:

```bash
docker run -p 8080:8080 \
  -e SECRET_KEY=your-secret-key \
  -e BUCKET_NAME=your-bucket-name \
  -e TMDB_API_KEY=your-tmdb-key \
  -v ~/.config/gcloud:/home/appuser/.config/gcloud:ro \
  ghcr.io/eveenendaal/video-gallery:latest
```

You need to configure the environment variables listed above and set up default GCP credentials. Install the [Google Cloud SDK](https://cloud.google.com/sdk/) and run `gcloud auth login --update-adc`. Then mount the credentials directory into the container as shown above.

### Storage Bucket
The application assumes the Storage Bucket is stored as follows:

* Category
  * Gallery
    * Video.ext (`.mp4`, `.m4v`, `.webm`, `.mov`, or `.avi`)
    * Video.jpg (optional thumbnail; `.jpeg` and `.png` also work)

Here's a real example

* Movies
  * Movies
    * My Movie 1.mp4
    * My Movie 1.jpg
    * My Movie 2.mp4
    * My Movie 2.jpg
* Home Videos
  * Bob
    * Video of Bob 1.mp4
    * Video of Bob 2.mp4
  * Alice
    * Video of Alice 1.mp4
    * Video of Alice 2.mp4
    * Video of Alice 3.mp4

The code parses the bucket and creates a list of categories, galleries, and videos, pairing each video with the image that shares its name in the same folder. Objects at any other depth or with other extensions are ignored. The listing is cached for 5 minutes (admin changes clear the cache immediately).
