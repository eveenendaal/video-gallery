package application

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"video-gallery/internal/domain/gallery"
)

// ProgressCallback is a function that receives step-name and percentage progress updates
type ProgressCallback func(step string, progress int)

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
	send := func(step string, progress int) {
		if progressCb != nil {
			progressCb(step, progress)
		}
	}

	send("Setting up directories", 10)
	outputDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Derive thumbnail storage path from video path
	ext := filepath.Ext(videoPath)
	basePath := videoPath[:len(videoPath)-len(ext)]
	thumbnailPath := basePath + ".jpg"

	videoBaseName := safeFilename(videoPath)
	thumbnailBaseName := safeFilename(thumbnailPath)

	send("Clearing old thumbnail", 20)
	// Best-effort delete of any existing thumbnail; ignore errors
	_ = s.repo.DeleteObject(context.Background(), thumbnailPath)

	send("Downloading video", 30)
	tmpVideoPath := filepath.Join(outputDir, videoBaseName)
	if err := s.repo.DownloadObject(context.Background(), videoPath, tmpVideoPath); err != nil {
		return fmt.Errorf("error downloading video: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpVideoPath); err != nil {
			log.Printf("Warning: failed to remove temp file: %v", err)
		}
	}()

	send("Generating thumbnail", 60)
	tmpThumbnailPath := filepath.Join(outputDir, thumbnailBaseName)
	if err := s.processor.ExtractFrame(tmpVideoPath, tmpThumbnailPath, timeMs); err != nil {
		return fmt.Errorf("error creating thumbnail: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpThumbnailPath); err != nil {
			log.Printf("Warning: failed to remove temp file: %v", err)
		}
	}()

	send("Validating thumbnail", 80)
	if err := s.processor.ValidateImage(tmpThumbnailPath); err != nil {
		return fmt.Errorf("thumbnail validation failed: %v", err)
	}

	send("Uploading thumbnail", 85)
	if err := s.repo.UploadObject(context.Background(), tmpThumbnailPath, thumbnailPath); err != nil {
		return fmt.Errorf("error uploading thumbnail: %v", err)
	}

	send("Clearing cache", 95)
	s.galleryService.InvalidateCache()

	send("Complete", 100)
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
// Returns (processed, errors, error).
func (s *ThumbnailService) BulkGenerateThumbnails(timeMs int, force bool) (int, int, error) {
	outputDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create output directory: %v", err)
	}

	ctx := context.Background()
	objects, err := s.repo.ListObjects(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list storage objects: %v", err)
	}

	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}

	// First pass: collect which base paths already have a thumbnail
	thumbnailsMap := make(map[string]bool)
	for _, obj := range objects {
		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}
		filename := parts[2]
		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				thumbnailsMap[obj.Name[:len(obj.Name)-len(filepath.Ext(obj.Name))]] = true
				break
			}
		}
	}

	totalProcessed := 0
	totalErrors := 0

	// Second pass: generate thumbnails for videos that need one
	for _, obj := range objects {
		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		filename := parts[2]
		isVideo := false
		for _, ext := range videoExtensions {
			if strings.HasSuffix(filename, ext) {
				isVideo = true
				break
			}
		}
		if !isVideo {
			continue
		}

		videoPath := obj.Name
		basePath := videoPath[:len(videoPath)-len(filepath.Ext(videoPath))]
		if thumbnailsMap[basePath] && !force {
			continue
		}

		thumbnailPath := basePath + ".jpg"
		videoBaseName := safeFilename(videoPath)
		thumbnailBaseName := safeFilename(thumbnailPath)

		tmpVideoPath := filepath.Join(outputDir, videoBaseName)
		if err := s.repo.DownloadObject(ctx, videoPath, tmpVideoPath); err != nil {
			log.Printf("Error downloading video %s: %v", videoPath, err)
			totalErrors++
			continue
		}

		tmpThumbnailPath := filepath.Join(outputDir, thumbnailBaseName)
		if err := s.processor.ExtractFrame(tmpVideoPath, tmpThumbnailPath, timeMs); err != nil {
			log.Printf("Error creating thumbnail for %s: %v", videoPath, err)
			os.Remove(tmpVideoPath)
			totalErrors++
			continue
		}

		if err := s.processor.ValidateImage(tmpThumbnailPath); err != nil {
			log.Printf("Thumbnail validation failed for %s: %v", videoPath, err)
			os.Remove(tmpVideoPath)
			os.Remove(tmpThumbnailPath)
			totalErrors++
			continue
		}

		if err := s.repo.UploadObject(ctx, tmpThumbnailPath, thumbnailPath); err != nil {
			log.Printf("Error uploading thumbnail for %s: %v", videoPath, err)
			os.Remove(tmpVideoPath)
			os.Remove(tmpThumbnailPath)
			totalErrors++
			continue
		}

		totalProcessed++
		os.Remove(tmpVideoPath)
		os.Remove(tmpThumbnailPath)
	}

	s.galleryService.InvalidateCache()
	return totalProcessed, totalErrors, nil
}

// BulkClearThumbnails removes all thumbnails from storage
func (s *ThumbnailService) BulkClearThumbnails() (int, error) {
	ctx := context.Background()
	objects, err := s.repo.ListObjects(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list storage objects: %v", err)
	}

	imageExtensions := []string{".jpg", ".jpeg", ".png"}
	totalDeleted := 0

	for _, obj := range objects {
		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		filename := parts[2]
		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				if err := s.repo.DeleteObject(ctx, obj.Name); err != nil {
					log.Printf("Error deleting thumbnail %s: %v", obj.Name, err)
				} else {
					totalDeleted++
				}
				break
			}
		}
	}

	s.galleryService.InvalidateCache()
	return totalDeleted, nil
}
