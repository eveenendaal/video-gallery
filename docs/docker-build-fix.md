# Docker Build Documentation

## Current Build Process

The application uses a multi-stage Dockerfile optimized for Rust:

### Build Stages
1. **Backend Builder**: Compiles Rust application using Alpine Linux
2. **Final Stage**: Minimal Alpine image with runtime dependencies only

### Prerequisites
Frontend assets must be built **before** running Docker build:
```bash
npm install
npm run build  # Builds SCSS to CSS
```

This is required because npm/node has compatibility issues in Alpine containers during the Docker build.

### Building the Docker Image

```bash
# 1. Build frontend assets first
npm install && npm run build

# 2. Build Docker image
docker build -t video-gallery:latest .

# 3. Run the container
docker run -p 8080:8080 \
  -e SECRET_KEY=your-secret-key \
  -e BUCKET_NAME=your-bucket-name \
  -e GOOGLE_APPLICATION_CREDENTIALS=/app/creds.json \
  -e TMDB_API_KEY=your-tmdb-key \
  -v /path/to/creds.json:/app/creds.json:ro \
  video-gallery:latest
```

## Runtime Requirements

### Required Environment Variables
- `SECRET_KEY` - Unique string for URL path protection
- `BUCKET_NAME` - GCS bucket containing video files
- `GOOGLE_APPLICATION_CREDENTIALS` - Path to service account JSON key file

### Optional Environment Variables
- `PORT` - Server port (default: 8080)
- `TMDB_API_KEY` - TMDb API key for movie poster feature

### Runtime Dependencies
The final container includes:
- **CA Certificates**: For HTTPS requests to GCS and TMDb
- **FFmpeg**: For thumbnail generation from video frames

### Service Account Permissions
The GCS service account needs:
- `storage.objects.list` - List bucket contents
- `storage.objects.get` - Read object metadata (for signed URLs)
- `storage.objects.create` - Upload thumbnails
- `storage.objects.delete` - Clear thumbnails

## Optimization Features

### Binary Size Optimization
The Cargo.toml includes aggressive optimization settings:
```toml
[profile.release]
strip = true          # Remove debug symbols
lto = true           # Link-time optimization
codegen-units = 1    # Better optimization at slower compile time
opt-level = "z"      # Optimize for size
```

Result: ~9.8 MB release binary

### Multi-stage Build Benefits
- **Builder stage**: Includes compilation tools and dependencies
- **Final stage**: Only runtime dependencies and binary
- **Size reduction**: From ~1GB builder image to ~30MB final image

## Testing the Docker Image Locally

```bash
# Build the image
npm run build && docker build -t video-gallery:test .

# Run with test configuration
docker run -p 8080:8080 \
  -e SECRET_KEY=test-key \
  -e BUCKET_NAME=your-test-bucket \
  -e GOOGLE_APPLICATION_CREDENTIALS=/app/creds.json \
  -v ./creds.json:/app/creds.json:ro \
  video-gallery:test

# Verify endpoints
curl http://localhost:8080/test-key/index
curl http://localhost:8080/test-key/feed
curl http://localhost:8080/test-key/admin
```

## Cloud Run Deployment

The container is designed to run on Google Cloud Run:

1. **Service Account**: Attach a service account with bucket access permissions
2. **Environment Variables**: Set via Cloud Run console or gcloud CLI
3. **Credentials**: When running on Cloud Run with proper service account, you may not need `GOOGLE_APPLICATION_CREDENTIALS` as it will use the attached service account automatically

## Troubleshooting

### "public/styles.css not found"
**Solution**: Build frontend assets before Docker build:
```bash
npm install && npm run build
```

### "Failed to load configuration: BUCKET_NAME environment variable not set"
**Solution**: Ensure all required environment variables are set when running the container

### "connection closed via error" during signed URL generation
**Solution**: This can happen with too many concurrent requests. The application limits concurrency to 50 to prevent this, but extremely large buckets (1000+ objects) may still encounter occasional errors. The application will log errors but continue processing other files.
