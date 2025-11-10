# Docker Build Fix Documentation

## Problem Summary

The GitHub Actions workflow in `.github/workflows/deploy.yml` was experiencing "manifest unknown" errors when building and publishing Docker images to GitHub Container Registry (GHCR).

## Root Causes Identified

### 1. Missing CA Certificates in Builder Stage
**Issue**: Alpine Linux base images don't include CA certificates by default
- This prevented Go from downloading modules securely via HTTPS
- Would cause TLS certificate verification failures

**Fix**: Added `RUN apk add --no-cache ca-certificates` in the builder stage before `go mod download`

### 2. Redundant Frontend Build Stage
**Issue**: The Dockerfile had a separate Node.js stage that rebuilt frontend assets
- This duplicated work already done by the GitHub Actions workflow
- Could cause inconsistencies between workflow-built and Docker-built assets
- Increased build time unnecessarily

**Fix**: Removed the frontend-builder stage and added a comment explaining that frontend assets should be pre-built by the CI workflow

## Changes Made

### Dockerfile Changes (`build/Dockerfile`)

```dockerfile
# Before - Missing CA certificates
FROM golang:1.25-alpine3.22 AS builder
WORKDIR /build

# After - Added CA certificates for secure module downloads
FROM golang:1.25-alpine3.22 AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /build
```

Removed entire frontend-builder stage:
```dockerfile
# REMOVED - Redundant with workflow build
FROM node:24-alpine AS frontend-builder
WORKDIR /frontend
COPY package*.json ./
RUN npm ci --omit=dev
COPY assets/scss ./assets/scss
RUN npm run build
```

## How the Build Process Works Now

### 1. GitHub Actions Workflow (deploy.yml)
```yaml
- name: Build Styles
  run: npm run build
```
This step compiles SCSS to CSS in the `public/` directory BEFORE Docker build

### 2. Docker Build
The Dockerfile now expects `public/` directory to already contain the compiled CSS:
```dockerfile
COPY . .
# Note: Frontend assets (public/) should be built by the CI workflow
# and copied into the build context before docker build is invoked
```

### 3. Multi-Platform Build
The workflow uses Docker Buildx to build for both AMD64 and ARM64:
```yaml
platforms: linux/amd64,linux/arm64
```

## Why "Manifest Unknown" Errors Occur

"Manifest unknown" errors typically happen when:

1. **Image doesn't exist**: The tag you're trying to pull hasn't been pushed successfully
2. **Wrong image name**: The image name format is incorrect (should be `ghcr.io/owner/repo:tag`)
3. **Authentication issues**: Not logged in or insufficient permissions
4. **Build failures**: The build failed but the push step tried to run anyway
5. **Tag mismatch**: Pushing one tag but trying to pull a different tag
6. **Multi-arch issues**: Expecting a manifest list but only single-platform images were pushed

## Validation Checklist

After these changes, the build should succeed if:

- ✅ Go version in Dockerfile matches go.mod
- ✅ Frontend assets are built in workflow before Docker build
- ✅ CA certificates are installed for secure module downloads
- ✅ Docker Buildx is set up for multi-platform builds
- ✅ Authentication to GHCR is configured with proper permissions
- ✅ Image naming follows the pattern: `ghcr.io/${{ github.repository_owner }}/repo-name:tag`

## Testing Locally

To test the Docker build locally:

```bash
# 1. Build frontend assets first
npm ci
npm run build

# 2. Build Docker image
docker build -f build/Dockerfile -t video-gallery:test --build-arg VERSION=test .

# 3. Run the container
docker run -p 8080:8080 \
  -e BUCKET_NAME=your-bucket \
  -e SECRET_KEY=your-secret \
  video-gallery:test
```

## Expected Behavior

After these fixes:

1. The GitHub Actions workflow will build successfully
2. Docker images will be pushed to `ghcr.io/eveenendaal/video-gallery` with multiple tags:
   - `latest`
   - Version number (e.g., `1.2.3`)
   - Major.minor version (e.g., `1.2`)
   - Major version (e.g., `1`)
   - Branch name (e.g., `master`)
3. Images will support both AMD64 and ARM64 architectures
4. The manifest will be available immediately after push
5. Users can pull the image: `docker pull ghcr.io/eveenendaal/video-gallery:latest`

## Additional Notes

### GitHub Container Registry Authentication
The workflow uses `GITHUB_TOKEN` for authentication, which automatically has the required `packages: write` permission specified in the workflow.

### Multi-Platform Builds
Docker Buildx handles creating the manifest list automatically when building for multiple platforms. The manifest includes references to both AMD64 and ARM64 images.

### Caching
The workflow uses GitHub Actions cache for Docker layers:
```yaml
cache-from: type=gha
cache-to: type=gha,mode=max
```
This speeds up subsequent builds significantly.

## Troubleshooting

If you still encounter "manifest unknown" errors after these changes:

1. **Check build logs**: Verify the build step completed successfully
2. **Verify push**: Look for "pushing manifest" messages in the logs
3. **Check permissions**: Ensure `packages: write` permission is set
4. **Verify authentication**: Check that `docker/login-action` succeeded
5. **Check image name**: Ensure it matches `ghcr.io/${{ github.repository }}`
6. **Wait briefly**: Sometimes there's a short delay in manifest propagation (seconds)

## References

- [GitHub Container Registry Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
- [Docker Buildx Multi-platform](https://docs.docker.com/build/building/multi-platform/)
- [Go Official Docker Images](https://hub.docker.com/_/golang)
