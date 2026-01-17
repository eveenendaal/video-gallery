package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

const (
	// colorDifferenceThreshold defines the minimum difference between color components
	// to consider two pixels as different colors (accounts for compression artifacts)
	colorDifferenceThreshold = uint32(256) // About 1 unit difference in 8-bit color
)

// ProgressCallback is a function that receives progress updates
type ProgressCallback func(step string, progress int)

// GenerateThumbnail generates a thumbnail for a specific video
func (s *Service) GenerateThumbnail(videoPath string, timeMs int) error {
	return s.GenerateThumbnailWithProgress(videoPath, timeMs, nil)
}

// GenerateThumbnailWithProgress generates a thumbnail with progress updates
func (s *Service) GenerateThumbnailWithProgress(videoPath string, timeMs int, progressCb ProgressCallback) error {
	sendProgress := func(step string, progress int) {
		if progressCb != nil {
			progressCb(step, progress)
		}
	}

	sendProgress("Checking FFmpeg", 5)
	if err := checkFFmpeg(); err != nil {
		return fmt.Errorf("FFmpeg is required but not found: %v", err)
	}

	sendProgress("Setting up directories", 10)
	outputDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	sendProgress("Connecting to storage", 15)
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.BucketName)

	// Generate thumbnail path
	ext := filepath.Ext(videoPath)
	basePath := videoPath[:len(videoPath)-len(ext)]
	thumbnailPath := basePath + ".jpg"

	// Generate safe filenames
	videoBaseName := getSafeFilename(videoPath)
	thumbnailBaseName := getSafeFilename(thumbnailPath)

	sendProgress("Clearing old thumbnail", 20)
	bucket.Object(thumbnailPath).Delete(ctx)

	sendProgress("Downloading video", 30)
	tmpVideoPath := filepath.Join(outputDir, videoBaseName)
	if err := downloadFile(ctx, bucket, videoPath, tmpVideoPath); err != nil {
		return fmt.Errorf("error downloading video: %v", err)
	}
	defer os.Remove(tmpVideoPath)

	sendProgress("Generating thumbnail", 60)
	tmpThumbnailPath := filepath.Join(outputDir, thumbnailBaseName)
	if err := createThumbnailWithFFmpeg(tmpVideoPath, tmpThumbnailPath, timeMs); err != nil {
		return fmt.Errorf("error creating thumbnail: %v", err)
	}
	defer os.Remove(tmpThumbnailPath)

	sendProgress("Validating thumbnail", 80)
	if err := validateThumbnail(tmpThumbnailPath); err != nil {
		return fmt.Errorf("thumbnail validation failed: %v", err)
	}

	sendProgress("Uploading thumbnail", 85)
	if err := uploadFile(ctx, bucket, tmpThumbnailPath, thumbnailPath); err != nil {
		return fmt.Errorf("error uploading thumbnail: %v", err)
	}

	sendProgress("Clearing cache", 95)
	s.videoCache.Flush()

	sendProgress("Complete", 100)
	return nil
}

// ClearThumbnail removes a thumbnail from storage
func (s *Service) ClearThumbnail(thumbnailPath string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.BucketName)

	// Delete the thumbnail
	if err := bucket.Object(thumbnailPath).Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete thumbnail: %v", err)
	}

	// Clear cache so thumbnail removal is visible
	s.videoCache.Flush()

	return nil
}

// BulkGenerateThumbnails generates thumbnails for all videos
func (s *Service) BulkGenerateThumbnails(timeMs int, force bool) (int, int, error) {
	// Check if ffmpeg is installed
	if err := checkFFmpeg(); err != nil {
		return 0, 0, fmt.Errorf("FFmpeg is required but not found: %v", err)
	}

	outputDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create output directory: %v", err)
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.BucketName)

	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}

	// Map to track which videos have thumbnails
	thumbnailsMap := make(map[string]bool)

	// First pass: find all thumbnails
	it := bucket.Objects(ctx, nil)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("error iterating objects: %v", err)
		}

		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		filename := parts[2]
		isImage := false
		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				isImage = true
				break
			}
		}

		if isImage {
			thumbnailsMap[obj.Name[:len(obj.Name)-len(filepath.Ext(obj.Name))]] = true
		}
	}

	// Second pass: find all videos and generate thumbnails
	it = bucket.Objects(ctx, nil)

	totalProcessed := 0
	totalErrors := 0

	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return totalProcessed, totalErrors, fmt.Errorf("error iterating objects: %v", err)
		}

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
		thumbnailNeeded := !thumbnailsMap[basePath] || force

		if thumbnailNeeded {
			thumbnailPath := basePath + ".jpg"

			// Generate safe filenames
			videoBaseName := getSafeFilename(videoPath)
			thumbnailBaseName := getSafeFilename(thumbnailPath)

			// Download video
			tmpVideoPath := filepath.Join(outputDir, videoBaseName)
			if err := downloadFile(ctx, bucket, videoPath, tmpVideoPath); err != nil {
				log.Printf("Error downloading video %s: %v", videoPath, err)
				totalErrors++
				continue
			}

			// Create thumbnail
			tmpThumbnailPath := filepath.Join(outputDir, thumbnailBaseName)
			if err := createThumbnailWithFFmpeg(tmpVideoPath, tmpThumbnailPath, timeMs); err != nil {
				log.Printf("Error creating thumbnail for %s: %v", videoPath, err)
				os.Remove(tmpVideoPath)
				totalErrors++
				continue
			}

			// Validate thumbnail
			if err := validateThumbnail(tmpThumbnailPath); err != nil {
				log.Printf("Thumbnail validation failed for %s: %v", videoPath, err)
				os.Remove(tmpVideoPath)
				os.Remove(tmpThumbnailPath)
				totalErrors++
				continue
			}

			// Upload thumbnail
			if err := uploadFile(ctx, bucket, tmpThumbnailPath, thumbnailPath); err != nil {
				log.Printf("Error uploading thumbnail for %s: %v", videoPath, err)
				os.Remove(tmpVideoPath)
				os.Remove(tmpThumbnailPath)
				totalErrors++
				continue
			}

			totalProcessed++

			// Clean up
			os.Remove(tmpVideoPath)
			os.Remove(tmpThumbnailPath)
		}
	}

	// Clear cache
	s.videoCache.Flush()

	return totalProcessed, totalErrors, nil
}

// BulkClearThumbnails removes all thumbnails from storage
func (s *Service) BulkClearThumbnails() (int, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.BucketName)

	imageExtensions := []string{".jpg", ".jpeg", ".png"}

	// Find and delete all thumbnails
	it := bucket.Objects(ctx, nil)
	totalDeleted := 0

	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return totalDeleted, fmt.Errorf("error iterating objects: %v", err)
		}

		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		filename := parts[2]
		isImage := false
		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				isImage = true
				break
			}
		}

		if isImage {
			if err := bucket.Object(obj.Name).Delete(ctx); err != nil {
				log.Printf("Error deleting thumbnail %s: %v", obj.Name, err)
				continue
			}
			totalDeleted++
		}
	}

	// Clear cache
	s.videoCache.Flush()

	return totalDeleted, nil
}

// Helper functions

func checkFFmpeg() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg not found or not working: %v", err)
	}
	return nil
}

func createThumbnailWithFFmpeg(videoPath, thumbnailPath string, timeMs int) error {
	// Convert milliseconds to HH:MM:SS.mmm format
	totalSeconds := timeMs / 1000
	milliseconds := timeMs % 1000

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	timeStr := fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)

	cmd := exec.Command(
		"ffmpeg",
		"-ss", timeStr,
		"-i", videoPath,
		"-vf", "thumbnail",
		"-frames:v", "1",
		"-q:v", "2",
		"-y",
		thumbnailPath,
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

func downloadFile(ctx context.Context, bucket *storage.BucketHandle, src, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer f.Close()

	if strings.HasPrefix(src, "http") {
		resp, err := http.Get(src)
		if err != nil {
			return fmt.Errorf("http.Get(%q): %v", src, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status code: %d", resp.StatusCode)
		}

		if _, err := io.Copy(f, resp.Body); err != nil {
			return fmt.Errorf("io.Copy: %v", err)
		}

		return nil
	}

	src = strings.TrimPrefix(src, "/")

	reader, err := bucket.Object(src).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", src, err)
	}
	defer reader.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("ReadFrom: %v", err)
	}

	return nil
}

func uploadFile(ctx context.Context, bucket *storage.BucketHandle, src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("os.ReadFile: %v", err)
	}

	dst = strings.TrimPrefix(dst, "/")

	if idx := strings.Index(dst, "?"); idx != -1 {
		dst = dst[:idx]
	}

	writer := bucket.Object(dst).NewWriter(ctx)
	writer.ContentType = "image/jpeg"

	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("Writer.Write: %v", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}

	return nil
}

func validateThumbnail(thumbnailPath string) error {
	f, err := os.Open(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to open thumbnail: %v", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to decode thumbnail: %v", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	sampleSize := 10
	stepX := width / sampleSize
	stepY := height / sampleSize

	if stepX == 0 {
		stepX = 1
	}
	if stepY == 0 {
		stepY = 1
	}

	firstColor := img.At(bounds.Min.X, bounds.Min.Y)
	r1, g1, b1, a1 := firstColor.RGBA()

	differentPixels := 0
	totalSamples := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			totalSamples++
			r2, g2, b2, a2 := img.At(x, y).RGBA()

			// If any color component differs by more than the threshold, count it as different
			if abs(int(r1)-int(r2)) > int(colorDifferenceThreshold) ||
				abs(int(g1)-int(g2)) > int(colorDifferenceThreshold) ||
				abs(int(b1)-int(b2)) > int(colorDifferenceThreshold) ||
				abs(int(a1)-int(a2)) > int(colorDifferenceThreshold) {
				differentPixels++
			}
		}
	}

	if totalSamples > 0 && float64(differentPixels)/float64(totalSamples) < 0.01 {
		return fmt.Errorf("thumbnail appears to be a solid color (only %d/%d sampled pixels differ)", differentPixels, totalSamples)
	}

	return nil
}

func getSafeFilename(path string) string {
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	baseName := filepath.Base(path)

	if len(baseName) > 200 {
		hash := sha256.Sum256([]byte(path))
		extension := filepath.Ext(baseName)

		shortName := baseName
		if len(baseName) > 20 {
			shortName = baseName[:20]
		}

		shortName = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`<>:"/\|?*`, r) {
				return '_'
			}
			return r
		}, shortName)

		baseName = fmt.Sprintf("%s-%s%s", shortName, hex.EncodeToString(hash[:8]), extension)
	}

	return baseName
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
