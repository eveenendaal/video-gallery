package application

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"video-gallery/internal/domain/gallery"
)

// ProgressCallback is a function that receives step-name and percentage progress updates
type ProgressCallback func(step string, progress int)

// report invokes cb if it is set
func (cb ProgressCallback) report(step string, progress int) {
	if cb != nil {
		cb(step, progress)
	}
}

// ThumbnailService handles thumbnail generation and management
type ThumbnailService struct {
	repo           gallery.StorageRepository
	processor      gallery.VideoProcessor
	galleryService *GalleryService
}

// NewThumbnailService creates a new ThumbnailService
func NewThumbnailService(
	repo gallery.StorageRepository,
	processor gallery.VideoProcessor,
	gallerySvc *GalleryService,
) *ThumbnailService {
	return &ThumbnailService{
		repo:           repo,
		processor:      processor,
		galleryService: gallerySvc,
	}
}

// GenerateThumbnail generates a thumbnail for a specific video with progress updates
func (s *ThumbnailService) GenerateThumbnail(videoPath string, timeMs int, progressCb ProgressCallback) error {
	if err := s.generate(context.Background(), videoPath, timeMs, progressCb); err != nil {
		return err
	}
	progressCb.report("Clearing cache", 95)
	s.galleryService.InvalidateCache()
	progressCb.report("Complete", 100)
	return nil
}

// generate downloads videoPath, extracts and validates a frame at timeMs, and
// uploads it as the video's .jpg thumbnail (replacing any existing one).
func (s *ThumbnailService) generate(ctx context.Context, videoPath string, timeMs int, progressCb ProgressCallback) error {
	progressCb.report("Downloading video", 30)
	tmpVideo, err := newTempFile(filepath.Ext(videoPath))
	if err != nil {
		return err
	}
	defer removeTempFile(tmpVideo)
	if err := s.repo.DownloadObject(ctx, videoPath, tmpVideo); err != nil {
		return fmt.Errorf("error downloading video: %v", err)
	}

	progressCb.report("Generating thumbnail", 60)
	tmpThumb, err := newTempFile(".jpg")
	if err != nil {
		return err
	}
	defer removeTempFile(tmpThumb)
	if err := s.processor.ExtractFrame(tmpVideo, tmpThumb, timeMs); err != nil {
		return fmt.Errorf("error creating thumbnail: %v", err)
	}

	progressCb.report("Validating thumbnail", 80)
	if err := s.processor.ValidateImage(tmpThumb); err != nil {
		return fmt.Errorf("thumbnail validation failed: %v", err)
	}

	progressCb.report("Uploading thumbnail", 85)
	if err := s.repo.UploadObject(ctx, tmpThumb, gallery.ThumbnailPathFor(videoPath)); err != nil {
		return fmt.Errorf("error uploading thumbnail: %v", err)
	}
	return nil
}

// ClearThumbnail removes a thumbnail from storage
func (s *ThumbnailService) ClearThumbnail(thumbnailPath string) error {
	if err := s.repo.DeleteObject(context.Background(), thumbnailPath); err != nil {
		return fmt.Errorf("failed to delete thumbnail: %v", err)
	}
	s.galleryService.InvalidateCache()
	return nil
}

// BulkGenerateThumbnails generates thumbnails for all videos that are missing one.
// When force is true, existing thumbnails are regenerated.
// Returns the number of thumbnails generated and the number of failures.
func (s *ThumbnailService) BulkGenerateThumbnails(timeMs int, force bool) (processed, failed int, err error) {
	ctx := context.Background()
	objects, err := s.repo.ListObjects(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list storage objects: %v", err)
	}

	hasThumbnail := make(map[string]bool)
	for _, obj := range objects {
		if _, ok := gallery.ParseObjectPath(obj.Name); ok && gallery.IsImage(obj.Name) {
			hasThumbnail[gallery.StripExt(obj.Name)] = true
		}
	}

	for _, obj := range objects {
		if _, ok := gallery.ParseObjectPath(obj.Name); !ok || !gallery.IsVideo(obj.Name) {
			continue
		}
		if hasThumbnail[gallery.StripExt(obj.Name)] && !force {
			continue
		}
		if err := s.generate(ctx, obj.Name, timeMs, nil); err != nil {
			log.Printf("Thumbnail generation failed for %s: %v", obj.Name, err)
			failed++
			continue
		}
		processed++
	}

	s.galleryService.InvalidateCache()
	return processed, failed, nil
}

// BulkClearThumbnails removes all thumbnails from storage
func (s *ThumbnailService) BulkClearThumbnails() (int, error) {
	ctx := context.Background()
	objects, err := s.repo.ListObjects(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list storage objects: %v", err)
	}

	deleted := 0
	for _, obj := range objects {
		if _, ok := gallery.ParseObjectPath(obj.Name); !ok || !gallery.IsImage(obj.Name) {
			continue
		}
		if err := s.repo.DeleteObject(ctx, obj.Name); err != nil {
			log.Printf("Error deleting thumbnail %s: %v", obj.Name, err)
			continue
		}
		deleted++
	}

	s.galleryService.InvalidateCache()
	return deleted, nil
}

// newTempFile reserves a uniquely named empty file in the work directory, so
// concurrent operations on same-named videos in different galleries never
// share a temp path.
func newTempFile(ext string) (string, error) {
	if err := os.MkdirAll(gallery.WorkDir(), 0o755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %v", err)
	}
	f, err := os.CreateTemp(gallery.WorkDir(), "*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	f.Close()
	return f.Name(), nil
}

func removeTempFile(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove temp file: %v", err)
	}
}
