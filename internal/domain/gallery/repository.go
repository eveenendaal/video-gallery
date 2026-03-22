package gallery

import (
	"context"
	"time"
)

// StorageRepository defines the contract for cloud storage operations used by the gallery domain
type StorageRepository interface {
	ListObjects(ctx context.Context) ([]StorageObject, error)
	GetSignedURL(ctx context.Context, path string, expiry time.Duration) (string, error)
	DeleteObject(ctx context.Context, path string) error
	DownloadObject(ctx context.Context, remotePath, localPath string) error
	UploadObject(ctx context.Context, localPath, remotePath string) error
}

// VideoProcessor defines the contract for video frame extraction and image validation
type VideoProcessor interface {
	ExtractFrame(videoPath, thumbnailPath string, timeMs int) error
	ValidateImage(imagePath string) error
}

// MovieResult represents a movie found via an external movie database
type MovieResult struct {
	ID          int
	Title       string
	PosterPath  *string
	ReleaseDate string
}

// MoviePosterClient defines the contract for movie poster lookup and download operations
type MoviePosterClient interface {
	SearchMovies(ctx context.Context, title string) ([]MovieResult, error)
	DownloadImage(ctx context.Context, imageURL, destPath string) error
}
