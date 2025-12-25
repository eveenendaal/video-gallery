package cmd

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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"video-gallery/pkg/config"
	"video-gallery/pkg/services"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// Command options
var (
	outputDir       string
	forceRegenerate bool
	frameTimeMs     int // Time in milliseconds where to extract the frame
	maxSizeMB       int // Maximum video size in MB to process
)

// newGenerateThumbnailsCmd creates a new command for generating thumbnails for videos
func newGenerateThumbnailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-thumbnails",
		Short: "Generate thumbnails for videos without existing thumbnails",
		Long:  `Generate thumbnails for videos that don't have existing thumbnails in the gallery.`,
		Run: func(cmd *cobra.Command, args []string) {
			// For thumbnail generation, we only need BUCKET_NAME, not SECRET_KEY
			// So we load config with a lenient approach
			bucketName := os.Getenv("BUCKET_NAME")
			if bucketName == "" {
				log.Fatalf("BUCKET_NAME environment variable not set")
			}

			// Create a minimal config just for thumbnail generation
			cfg := &config.Config{
				BucketName: bucketName,
				SecretKey:  "", // Not needed for thumbnail generation
				Port:       "8080",
			}
			services.InitService(cfg)
			generateThumbnails()
		},
	}

	// Add command-specific flags
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "thumbnails", "Directory to store temporary thumbnails")
	cmd.Flags().BoolVarP(&forceRegenerate, "force", "f", false, "Force regeneration of all thumbnails, even if they exist")
	cmd.Flags().IntVarP(&frameTimeMs, "time", "t", 1000, "Time in milliseconds where to extract the thumbnail frame")
	cmd.Flags().IntVarP(&maxSizeMB, "max-size", "m", 1024, "Maximum video size in MB to process (0 means no limit)")

	return cmd
}

// generateThumbnails creates thumbnails for videos that don't have them
func generateThumbnails() {
	// Check if ffmpeg is installed
	if err := checkFFmpeg(); err != nil {
		log.Fatalf("FFmpeg is required but not found: %v", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Warning: error closing storage client: %v", err)
		}
	}()

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		log.Fatalf("BUCKET_NAME environment variable not set")
	}

	bucket := client.Bucket(bucketName)

	// Scan bucket directly to find videos and existing thumbnails
	fmt.Println("Scanning bucket for videos without thumbnails...")

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
			log.Printf("Error iterating objects: %v", err)
			continue
		}

		// Check if file has structure: Category/Gallery/File
		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		filename := parts[2]

		// Check if it's an image (thumbnail)
		isImage := false
		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				isImage = true
				break
			}
		}

		if isImage {
			// Store the base path without extension
			thumbnailsMap[obj.Name[:len(obj.Name)-len(filepath.Ext(obj.Name))]] = true
		}
	}

	// Second pass: find all videos and check if they need thumbnails
	it = bucket.Objects(ctx, nil)

	totalVideos := 0
	totalProcessed := 0
	missingThumbnails := 0

	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Error iterating objects: %v", err)
			continue
		}

		// Check if file has structure: Category/Gallery/File
		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		category := parts[0]
		gallery := parts[1]
		filename := parts[2]

		// Check if it's a video
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

		totalVideos++
		videoPath := obj.Name

		// Get base name without extension
		basePath := videoPath[:len(videoPath)-len(filepath.Ext(videoPath))]

		// Check if thumbnail exists
		thumbnailNeeded := !thumbnailsMap[basePath] || forceRegenerate

		if thumbnailNeeded {
			missingThumbnails++
			fmt.Printf("Gallery: %s/%s\n", category, gallery)
			fmt.Printf("  Generating thumbnail for: %s\n", filename)

			thumbnailPath := basePath + ".jpg"

			// Check file size before downloading if max size limit is set
			if maxSizeMB > 0 {
				videoSizeMB := obj.Size / (1024 * 1024)

				if videoSizeMB > int64(maxSizeMB) {
					fmt.Printf("    Skipping video %s: size %d MB exceeds limit of %d MB\n",
						filename, videoSizeMB, maxSizeMB)
					continue
				}
			}

			// Generate safe filenames for local storage
			videoBaseName := getSafeFilename(videoPath)
			thumbnailBaseName := getSafeFilename(thumbnailPath)

			// Download video to temp location with safe filename
			tmpVideoPath := filepath.Join(outputDir, videoBaseName)
			if err := downloadFile(ctx, bucket, videoPath, tmpVideoPath); err != nil {
				fmt.Printf("    Error downloading video: %v\n", err)
				continue
			}

			// Create thumbnail using FFmpeg with safe filename
			tmpThumbnailPath := filepath.Join(outputDir, thumbnailBaseName)
			if err := createThumbnailWithFFmpeg(tmpVideoPath, tmpThumbnailPath); err != nil {
				fmt.Printf("    Error creating thumbnail: %v\n", err)
				// Clean up video file
				if err := os.Remove(tmpVideoPath); err != nil {
					log.Printf("Warning: failed to remove temp file %s: %v", tmpVideoPath, err)
				}
				continue
			}

			// Validate the thumbnail isn't just a solid color
			if err := validateThumbnail(tmpThumbnailPath); err != nil {
				fmt.Printf("    Warning: thumbnail validation failed: %v\n", err)
				// Clean up files
				if err := os.Remove(tmpVideoPath); err != nil {
					log.Printf("Warning: failed to remove temp file %s: %v", tmpVideoPath, err)
				}
				if err := os.Remove(tmpThumbnailPath); err != nil {
					log.Printf("Warning: failed to remove temp file %s: %v", tmpThumbnailPath, err)
				}
				continue
			}

			// Upload thumbnail to bucket
			if err := uploadFile(ctx, bucket, tmpThumbnailPath, thumbnailPath); err != nil {
				fmt.Printf("    Error uploading thumbnail: %v\n", err)
				// Clean up files
				if err := os.Remove(tmpVideoPath); err != nil {
					log.Printf("Warning: failed to remove temp file %s: %v", tmpVideoPath, err)
				}
				if err := os.Remove(tmpThumbnailPath); err != nil {
					log.Printf("Warning: failed to remove temp file %s: %v", tmpThumbnailPath, err)
				}
				continue
			}

			fmt.Printf("    Created thumbnail: %s\n", thumbnailPath)
			totalProcessed++

			// Clean up temporary files
			if err := os.Remove(tmpVideoPath); err != nil {
				log.Printf("Warning: failed to remove temp file %s: %v", tmpVideoPath, err)
			}
			if err := os.Remove(tmpThumbnailPath); err != nil {
				log.Printf("Warning: failed to remove temp file %s: %v", tmpThumbnailPath, err)
			}
		}
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total videos: %d\n", totalVideos)
	fmt.Printf("  Videos without thumbnails: %d\n", missingThumbnails)
	fmt.Printf("  Thumbnails successfully generated: %d\n", totalProcessed)
}

// getSafeFilename creates a safe filename from a URL by:
// 1. Removing query parameters
// 2. Using only the base name
// 3. If still too long, using a hash of the original path
func getSafeFilename(path string) string {
	// Remove query parameters by taking everything before '?'
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	// Get just the filename without directory
	baseName := filepath.Base(path)

	// If the name is still too long (>200 chars is usually problematic)
	if len(baseName) > 200 {
		// Create a hash of the original path
		hash := sha256.Sum256([]byte(path))
		extension := filepath.Ext(baseName)

		// Use the first part of the filename (up to 20 chars) + hash + extension
		shortName := baseName
		if len(baseName) > 20 {
			shortName = baseName[:20]
		}

		// Remove characters that might be problematic in filenames
		shortName = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`<>:"/\|?*`, r) {
				return '_'
			}
			return r
		}, shortName)

		// Create a new filename with hash
		baseName = fmt.Sprintf("%s-%s%s", shortName, hex.EncodeToString(hash[:8]), extension)
	}

	return baseName
}

// generateThumbnailPath converts a video path to a thumbnail path
func generateThumbnailPath(videoPath string) string {
	// For URLs, extract just the bucket path part
	if strings.HasPrefix(videoPath, "http") {
		// If it's a URL, strip the domain and keep just the path
		// Example: "https://storage.googleapis.com/veenendaal-videos/Videos/SNL/SNL%2050.mp4"
		// Should become: "Videos/SNL/SNL 50.jpg"

		// Find the bucket name in the URL
		parts := strings.Split(videoPath, "/")
		bucketIndex := -1
		bucketName := os.Getenv("BUCKET_NAME")

		for i, part := range parts {
			if part == bucketName {
				bucketIndex = i
				break
			}
		}

		if bucketIndex >= 0 && bucketIndex+1 < len(parts) {
			// Extract just the path after the bucket name
			videoPath = strings.Join(parts[bucketIndex+1:], "/")
		}
	}

	// Remove any URL parameters
	if idx := strings.Index(videoPath, "?"); idx != -1 {
		videoPath = videoPath[:idx]
	}

	// URL-decode the path (convert %20 to spaces, etc.)
	var err error
	videoPath, err = url.QueryUnescape(videoPath)
	if err != nil {
		// If there's an error decoding, just use the original path
		log.Printf("Warning: couldn't decode URL path: %v", err)
	}

	ext := filepath.Ext(videoPath)
	baseName := videoPath[:len(videoPath)-len(ext)]
	return baseName + ".jpg"
}

// checkFFmpeg verifies that FFmpeg is installed and accessible
func checkFFmpeg() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg not found or not working: %v", err)
	}
	return nil
}

// createThumbnailWithFFmpeg creates a thumbnail from a video using FFmpeg
func createThumbnailWithFFmpeg(videoPath, thumbnailPath string) error {
	// Format time for FFmpeg (convert milliseconds to HH:MM:SS.mmm format)
	seconds := frameTimeMs / 1000
	milliseconds := frameTimeMs % 1000
	timeStr := fmt.Sprintf("00:00:%02d.%03d", seconds, milliseconds)

	// Use FFmpeg to extract a frame at the specified time
	// -ss before -i is fast, then read a few frames to get past any blank/grey frames
	cmd := exec.Command(
		"ffmpeg",
		"-ss", timeStr, // Seek before input (faster, approximate)
		"-i", videoPath, // Input file
		"-vf", "thumbnail", // Use thumbnail filter to select best frame
		"-frames:v", "1", // Extract only one frame
		"-q:v", "2", // Quality (lower is better, 2-5 is good for JPEG)
		"-y",          // Overwrite without asking
		thumbnailPath, // Output file
	)

	// Capture any error output for better error messages
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// downloadFile downloads a file from GCS bucket to a local path
// Handles both direct object paths and signed URLs
func downloadFile(ctx context.Context, bucket *storage.BucketHandle, src, dst string) error {
	// Create the file
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Warning: error closing file %s: %v", dst, err)
		}
	}()

	// Handle direct download of the content using the full URL
	// This is needed for signed URLs which can't be accessed through object methods
	if strings.HasPrefix(src, "http") {
		// Use http.Get for the signed URL
		resp, err := http.Get(src)
		if err != nil {
			return fmt.Errorf("http.Get(%q): %v", src, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status code: %d", resp.StatusCode)
		}

		// Get content length for progress reporting
		contentLength := resp.ContentLength
		baseName := filepath.Base(dst)

		// Create a progress tracking reader
		progressReader := &progressReader{
			reader:        resp.Body,
			contentLength: contentLength,
			fileName:      baseName,
			lastUpdate:    time.Now(),
			bytesRead:     0,
		}

		// Copy the response body to the local file with progress reporting
		if _, err := io.Copy(f, progressReader); err != nil {
			return fmt.Errorf("io.Copy: %v", err)
		}

		// Print a newline after download completes
		if contentLength > 0 {
			fmt.Println()
		}

		return nil
	}

	// For direct object paths (not URLs), use the standard GCS approach
	// Remove any leading slash from src
	src = strings.TrimPrefix(src, "/")

	// Download the file from the bucket
	reader, err := bucket.Object(src).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", src, err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("Warning: error closing reader: %v", err)
		}
	}()

	// Get content length for progress reporting
	size := reader.Attrs.Size
	baseName := filepath.Base(dst)

	// Create a progress tracking reader
	progressReader := &progressReader{
		reader:        reader,
		contentLength: size,
		fileName:      baseName,
		lastUpdate:    time.Now(),
		bytesRead:     0,
	}

	// Copy the data with progress reporting
	if _, err := io.Copy(f, progressReader); err != nil {
		return fmt.Errorf("ReadFrom: %v", err)
	}

	// Print a newline after download completes
	if size > 0 {
		fmt.Println()
	}

	return nil
}

// getVideoSize checks the size of a video file before downloading it
func getVideoSize(ctx context.Context, bucket *storage.BucketHandle, src string) (int64, error) {
	// For signed URLs, make a HEAD request to get the Content-Length
	if strings.HasPrefix(src, "http") {
		// Create a HEAD request to avoid downloading the content
		req, err := http.NewRequestWithContext(ctx, "HEAD", src, nil)
		if err != nil {
			return 0, fmt.Errorf("error creating HEAD request: %v", err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("error making HEAD request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("bad status code: %d", resp.StatusCode)
		}

		// Get Content-Length header
		contentLength := resp.ContentLength
		if contentLength <= 0 {
			// If Content-Length is not available, we can't determine size
			return 0, fmt.Errorf("content length not available for URL")
		}

		return contentLength, nil
	}

	// For direct object paths, use the GCS API
	src = strings.TrimPrefix(src, "/")
	attrs, err := bucket.Object(src).Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get object attributes: %v", err)
	}

	return attrs.Size, nil
}

// progressReader wraps an io.Reader to provide progress updates
type progressReader struct {
	reader        io.Reader
	contentLength int64
	fileName      string
	lastUpdate    time.Time
	bytesRead     int64
}

// Read implements the io.Reader interface
func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.bytesRead += int64(n)

	// Don't report progress too frequently - update at most every 500ms
	now := time.Now()
	if now.Sub(pr.lastUpdate) >= 500*time.Millisecond {
		pr.updateProgress()
		pr.lastUpdate = now
	}

	return n, err
}

// updateProgress prints the download progress
func (pr *progressReader) updateProgress() {
	if pr.contentLength <= 0 {
		// If content length is unknown, just show bytes read
		fmt.Printf("\r    Downloading %s: %d bytes...", pr.fileName, pr.bytesRead)
		return
	}

	// Calculate percentage
	percent := float64(pr.bytesRead) / float64(pr.contentLength) * 100

	// Format sizes in human-readable format
	downloaded := formatSize(pr.bytesRead)
	total := formatSize(pr.contentLength)

	// Update the progress line (overwrite previous with \r)
	fmt.Printf("\r    Downloading %s: %.1f%% (%s/%s)...", pr.fileName, percent, downloaded, total)
}

// formatSize converts bytes to a human-readable format
func formatSize(bytes int64) string {
	const (
		B  int64 = 1
		KB       = B * 1024
		MB       = KB * 1024
		GB       = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// uploadFile uploads a file to GCS bucket
func uploadFile(ctx context.Context, bucket *storage.BucketHandle, src, dst string) error {
	// Read the file data
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("os.ReadFile: %v", err)
	}

	// Process destination path
	// Remove any leading slash from dst
	dst = strings.TrimPrefix(dst, "/")

	// Remove query parameters if any
	if idx := strings.Index(dst, "?"); idx != -1 {
		dst = dst[:idx]
	}

	fmt.Printf("    Uploading thumbnail to %s... ", dst)

	// Create a writer with appropriate content type
	writer := bucket.Object(dst).NewWriter(ctx)
	writer.ContentType = "image/jpeg"

	// Write the file
	if _, err := writer.Write(data); err != nil {
		fmt.Println("Failed")
		return fmt.Errorf("Writer.Write: %v", err)
	}

	// Close the writer to complete the upload
	if err := writer.Close(); err != nil {
		fmt.Println("Failed")
		return fmt.Errorf("Writer.Close: %v", err)
	}

	fmt.Println("Done")
	return nil
}

// validateThumbnail checks if a thumbnail is valid (not a solid color)
func validateThumbnail(thumbnailPath string) error {
	// Open the image file
	f, err := os.Open(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to open thumbnail: %v", err)
	}
	defer f.Close()

	// Decode the image
	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to decode thumbnail: %v", err)
	}

	// Sample pixels to check for color variation
	// We'll sample a grid of points across the image
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Sample 100 points (10x10 grid)
	sampleSize := 10
	stepX := width / sampleSize
	stepY := height / sampleSize

	if stepX == 0 {
		stepX = 1
	}
	if stepY == 0 {
		stepY = 1
	}

	// Get the first pixel's color as reference
	firstColor := img.At(bounds.Min.X, bounds.Min.Y)
	r1, g1, b1, a1 := firstColor.RGBA()

	// Check if all sampled pixels are the same color
	differentPixels := 0
	totalSamples := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			totalSamples++
			r2, g2, b2, a2 := img.At(x, y).RGBA()

			// If any color component differs by more than a small threshold, count it as different
			// Using a threshold to account for compression artifacts
			threshold := uint32(256) // About 1 unit difference in 8-bit color
			if abs(int(r1)-int(r2)) > int(threshold) ||
				abs(int(g1)-int(g2)) > int(threshold) ||
				abs(int(b1)-int(b2)) > int(threshold) ||
				abs(int(a1)-int(a2)) > int(threshold) {
				differentPixels++
			}
		}
	}

	// If less than 1% of pixels are different, consider it a solid color
	if totalSamples > 0 && float64(differentPixels)/float64(totalSamples) < 0.01 {
		return fmt.Errorf("thumbnail appears to be a solid color (only %d/%d sampled pixels differ)", differentPixels, totalSamples)
	}

	return nil
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
