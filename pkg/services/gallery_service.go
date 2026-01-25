package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"video-gallery/pkg/config"

	"cloud.google.com/go/storage"
	"github.com/patrickmn/go-cache"
	"google.golang.org/api/iterator"

	"video-gallery/pkg/models"
)

// Service handles operations related to galleries and videos
type Service struct {
	config     *config.Config
	videoCache *cache.Cache
	mu         sync.RWMutex
}

// naturalLess compares strings in a way that treats numbers as numbers rather than characters
// For example: "file2" < "file10" when using naturalLess
func naturalLess(s1, s2 string) bool {
	i, j := 0, 0
	for i < len(s1) && j < len(s2) {
		// Skip leading spaces
		for i < len(s1) && unicode.IsSpace(rune(s1[i])) {
			i++
		}
		for j < len(s2) && unicode.IsSpace(rune(s2[j])) {
			j++
		}

		// If we reached the end of either string
		if i >= len(s1) || j >= len(s2) {
			break
		}

		// If both characters are digits, compare the numbers
		if unicode.IsDigit(rune(s1[i])) && unicode.IsDigit(rune(s2[j])) {
			// Extract consecutive digits
			var num1, num2 string
			for i < len(s1) && unicode.IsDigit(rune(s1[i])) {
				num1 += string(s1[i])
				i++
			}
			for j < len(s2) && unicode.IsDigit(rune(s2[j])) {
				num2 += string(s2[j])
				j++
			}

			// Convert to integers and compare
			n1, _ := strconv.Atoi(num1)
			n2, _ := strconv.Atoi(num2)
			if n1 != n2 {
				return n1 < n2
			}
			// If numbers are equal, continue to next characters
		} else {
			// Compare characters
			if s1[i] != s2[j] {
				return s1[i] < s2[j]
			}
			i++
			j++
		}
	}

	// If we've reached the end of one string but not the other
	return len(s1) < len(s2)
}

var (
	// defaultService is the singleton instance of Service
	defaultService *Service
	once           sync.Once
)

// InitService initializes the service with the given configuration
func InitService(cfg *config.Config) {
	once.Do(func() {
		defaultService = &Service{
			config:     cfg,
			videoCache: cache.New(5*time.Minute, 10*time.Minute),
		}
	})
}

// GetCategories returns all categories with their galleries
func GetCategories() []models.Category {
	return defaultService.GetCategoriesInternal()
}

// GetCategoriesInternal returns all categories with their galleries
func (s *Service) GetCategoriesInternal() []models.Category {
	galleries := s.GetGalleriesInternal()
	categoryMap := make(map[string]*models.Category)

	for _, gallery := range galleries {
		categoryName := gallery.Category
		if cat, exists := categoryMap[categoryName]; exists {
			cat.Galleries = append(cat.Galleries, gallery)
		} else {
			categoryMap[categoryName] = &models.Category{
				Name:      categoryName,
				Stub:      categoryName,
				Galleries: []models.Gallery{gallery},
			}
		}
	}

	// Convert map to slice
	categories := make([]models.Category, 0, len(categoryMap))
	for _, category := range categoryMap {
		categories = append(categories, *category)
	}

	return categories
}

// GetGallery returns a gallery by its stub
func GetGallery(stub string) (models.Gallery, error) {
	return defaultService.GetGalleryInternal(stub)
}

// GetGalleryInternal returns a gallery by its stub
func (s *Service) GetGalleryInternal(stub string) (models.Gallery, error) {
	galleries := s.GetGalleriesInternal()
	for _, gallery := range galleries {
		if gallery.Stub == stub {
			return gallery, nil
		}
	}
	return models.Gallery{}, fmt.Errorf("gallery not found: %s", stub)
}

// GetGalleries returns all galleries with their videos
func GetGalleries() []models.Gallery {
	return defaultService.GetGalleriesInternal()
}

// GetGalleriesInternal returns all galleries with their videos
func (s *Service) GetGalleriesInternal() []models.Gallery {
	videos := s.GetVideosInternal()
	galleryMap := make(map[string]*models.Gallery)

	for _, video := range videos {
		galleryName := video.Gallery
		if g, exists := galleryMap[galleryName]; exists {
			g.Videos = append(g.Videos, video)
		} else {
			// Generate Hash for gallery URL (non-security use case)
			// This hash is used only for generating short URL identifiers, not for security purposes
			hash := sha256.New()
			hash.Write([]byte(galleryName + s.config.SecretKey))
			hashStr := base64.URLEncoding.EncodeToString(hash.Sum(nil))[0:4]

			galleryMap[galleryName] = &models.Gallery{
				Name:     galleryName,
				Category: video.Category,
				Stub:     fmt.Sprintf("/gallery/%s", hashStr),
				Videos:   []models.Video{video},
			}
		}
	}

	// Convert the map to slice
	galleries := make([]models.Gallery, 0, len(galleryMap))
	for _, gallery := range galleryMap {
		galleries = append(galleries, *gallery)
	}

	// Sort galleries alphabetically by name with natural sorting for numbers
	sort.Slice(galleries, func(i, j int) bool {
		return naturalLess(galleries[i].Name, galleries[j].Name)
	})

	return galleries
}

// GetVideos returns all videos from the storage bucket
func GetVideos() []models.Video {
	return defaultService.GetVideosInternal()
}

// GenerateThumbnail generates a thumbnail for a specific video
func GenerateThumbnail(videoPath string, timeMs int) error {
	return defaultService.GenerateThumbnail(videoPath, timeMs)
}

// GenerateThumbnailWithProgress generates a thumbnail for a specific video with progress updates
func GenerateThumbnailWithProgress(videoPath string, timeMs int, progressCb func(string, int)) error {
	return defaultService.GenerateThumbnailWithProgress(videoPath, timeMs, progressCb)
}

// ClearThumbnail removes a thumbnail from storage
func ClearThumbnail(thumbnailPath string) error {
	return defaultService.ClearThumbnail(thumbnailPath)
}

// BulkGenerateThumbnails generates thumbnails for all videos
func BulkGenerateThumbnails(timeMs int, force bool) (int, int, error) {
	return defaultService.BulkGenerateThumbnails(timeMs, force)
}

// BulkClearThumbnails removes all thumbnails from storage
func BulkClearThumbnails() (int, error) {
	return defaultService.BulkClearThumbnails()
}

// FetchMoviePoster searches for a movie poster and uploads it to storage
func FetchMoviePoster(videoPath string, movieTitle string, progressCb ProgressCallback) error {
	return defaultService.FetchMoviePoster(videoPath, movieTitle, progressCb)
}

// SearchMoviePoster searches for a movie and returns available posters
func SearchMoviePoster(movieTitle string) ([]MoviePosterResult, error) {
	return defaultService.SearchMoviePoster(movieTitle)
}

// GetVideosInternal returns all videos from the storage bucket
func (s *Service) GetVideosInternal() []models.Video {
	s.mu.RLock()
	if cachedVideos, found := s.videoCache.Get("videos"); found {
		s.mu.RUnlock()
		log.Println("Using Cached Videos")
		return cachedVideos.([]models.Video)
	}
	s.mu.RUnlock()

	log.Println("Getting Videos")

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize Cloud Storage
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create storage client: %v", err)
		return []models.Video{}
	}
	defer storageClient.Close()

	bucket := storageClient.Bucket(s.config.BucketName)
	it := bucket.Objects(ctx, nil)
	videosMap := make(map[string]models.Video)

	// Allowed Extensions
	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}

	// Compile regex for file extension extraction
	extensionRegex, err := regexp.Compile(`\.[a-zA-Z0-9]+$`)
	if err != nil {
		log.Printf("Failed to compile regex: %v", err)
		return []models.Video{}
	}

	// Iterate through files
	for {
		file, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Printf("Error iterating objects: %v", err)
			continue
		}

		parts := strings.Split(file.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		category := parts[0]
		gallery := parts[1]
		filename := parts[2]

		// Create a Signed 24-Hour URL
		signedURL, err := bucket.SignedURL(file.Name, &storage.SignedURLOptions{
			Expires: time.Now().Add(24 * time.Hour),
			Method:  "GET",
		})
		if err != nil {
			log.Printf("Error creating signed URL for %s: %v", file.Name, err)
			continue
		}

		// Remove extension from the filename
		fileBase := string(extensionRegex.ReplaceAllString(filename, ""))

		// Initialize a video if it doesn't exist
		if _, ok := videosMap[fileBase]; !ok {
			videosMap[fileBase] = models.Video{
				Name:     fileBase,
				Category: category,
				Gallery:  gallery,
			}
		}

		// Update video with URL or thumbnail
		video := videosMap[fileBase]

		// Check if a file is a video
		for _, ext := range videoExtensions {
			if strings.HasSuffix(filename, ext) {
				video.Url = signedURL
				video.VideoPath = file.Name
				videosMap[fileBase] = video
				break
			}
		}

		// Check if file is an image (thumbnail)
		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				video.Thumbnail = &signedURL
				video.ThumbnailPath = file.Name
				videosMap[fileBase] = video
				break
			}
		}
	}

	// Convert map to slice
	videos := make([]models.Video, 0, len(videosMap))
	for _, video := range videosMap {
		videos = append(videos, video)
	}

	// Sort videos alphabetically by name with natural sorting for numbers
	sort.Slice(videos, func(i, j int) bool {
		return naturalLess(videos[i].Name, videos[j].Name)
	})

	// Cache videos
	s.mu.Lock()
	s.videoCache.Set("videos", videos, cache.DefaultExpiration)
	s.mu.Unlock()

	return videos
}
