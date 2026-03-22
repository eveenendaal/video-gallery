package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/patrickmn/go-cache"

	"video-gallery/internal/domain/gallery"
)

// GalleryService handles gallery and video retrieval with caching
type GalleryService struct {
	repo       gallery.StorageRepository
	secretKey  string
	videoCache *cache.Cache
	mu         sync.RWMutex
}

// NewGalleryService creates a new GalleryService
func NewGalleryService(repo gallery.StorageRepository, secretKey string) *GalleryService {
	return &GalleryService{
		repo:       repo,
		secretKey:  secretKey,
		videoCache: cache.New(5*time.Minute, 10*time.Minute),
	}
}

// GetCategories returns all categories with their galleries
func (s *GalleryService) GetCategories() []gallery.Category {
	galleries := s.GetGalleries()
	categoryMap := make(map[string]*gallery.Category)

	for _, g := range galleries {
		categoryName := g.Category
		if cat, exists := categoryMap[categoryName]; exists {
			cat.Galleries = append(cat.Galleries, g)
		} else {
			categoryMap[categoryName] = &gallery.Category{
				Name:      categoryName,
				Stub:      categoryName,
				Galleries: []gallery.Gallery{g},
			}
		}
	}

	categories := make([]gallery.Category, 0, len(categoryMap))
	for _, category := range categoryMap {
		categories = append(categories, *category)
	}
	return categories
}

// GetGallery returns a gallery by its stub
func (s *GalleryService) GetGallery(stub string) (gallery.Gallery, error) {
	galleries := s.GetGalleries()
	for _, g := range galleries {
		if g.Stub == stub {
			return g, nil
		}
	}
	return gallery.Gallery{}, fmt.Errorf("gallery not found: %s", stub)
}

// GetGalleries returns all galleries with their videos
func (s *GalleryService) GetGalleries() []gallery.Gallery {
	videos := s.GetVideos()
	galleryMap := make(map[string]*gallery.Gallery)

	for _, video := range videos {
		galleryName := video.Gallery
		if g, exists := galleryMap[galleryName]; exists {
			g.Videos = append(g.Videos, video)
		} else {
			// Generate hash for gallery URL (non-security use case — URL identifier only)
			hash := sha256.New()
			hash.Write([]byte(galleryName + s.secretKey))
			hashStr := base64.URLEncoding.EncodeToString(hash.Sum(nil))[0:4]

			galleryMap[galleryName] = &gallery.Gallery{
				Name:     galleryName,
				Category: video.Category,
				Stub:     fmt.Sprintf("/gallery/%s", hashStr),
				Videos:   []gallery.Video{video},
			}
		}
	}

	galleries := make([]gallery.Gallery, 0, len(galleryMap))
	for _, g := range galleryMap {
		galleries = append(galleries, *g)
	}

	sort.Slice(galleries, func(i, j int) bool {
		return naturalLess(galleries[i].Name, galleries[j].Name)
	})

	return galleries
}

// GetVideos returns all videos from storage, using a 5-minute cache
func (s *GalleryService) GetVideos() []gallery.Video {
	s.mu.RLock()
	if cachedVideos, found := s.videoCache.Get("videos"); found {
		s.mu.RUnlock()
		log.Println("Using Cached Videos")
		return cachedVideos.([]gallery.Video)
	}
	s.mu.RUnlock()

	log.Println("Getting Videos")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objects, err := s.repo.ListObjects(ctx)
	if err != nil {
		log.Printf("Failed to list storage objects: %v", err)
		return []gallery.Video{}
	}

	videosMap := make(map[string]gallery.Video)

	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}

	extensionRegex, err := regexp.Compile(`\.[a-zA-Z0-9]+$`)
	if err != nil {
		log.Printf("Failed to compile regex: %v", err)
		return []gallery.Video{}
	}

	for _, obj := range objects {
		parts := strings.Split(obj.Name, "/")
		if len(parts) != 3 || parts[2] == "" {
			continue
		}

		category := parts[0]
		galleryName := parts[1]
		filename := parts[2]

		signedURL, err := s.repo.GetSignedURL(ctx, obj.Name, 24*time.Hour)
		if err != nil {
			log.Printf("Error creating signed URL for %s: %v", obj.Name, err)
			continue
		}

		fileBase := extensionRegex.ReplaceAllString(filename, "")

		if _, ok := videosMap[fileBase]; !ok {
			videosMap[fileBase] = gallery.Video{
				Name:     fileBase,
				Category: category,
				Gallery:  galleryName,
			}
		}

		video := videosMap[fileBase]

		for _, ext := range videoExtensions {
			if strings.HasSuffix(filename, ext) {
				video.Url = signedURL
				video.VideoPath = obj.Name
				videosMap[fileBase] = video
				break
			}
		}

		for _, ext := range imageExtensions {
			if strings.HasSuffix(filename, ext) {
				video.Thumbnail = &signedURL
				video.ThumbnailPath = obj.Name
				videosMap[fileBase] = video
				break
			}
		}
	}

	videos := make([]gallery.Video, 0, len(videosMap))
	for _, video := range videosMap {
		videos = append(videos, video)
	}

	sort.Slice(videos, func(i, j int) bool {
		return naturalLess(videos[i].Name, videos[j].Name)
	})

	s.mu.Lock()
	s.videoCache.Set("videos", videos, cache.DefaultExpiration)
	s.mu.Unlock()

	return videos
}

// InvalidateCache clears the video cache so subsequent reads fetch fresh data
func (s *GalleryService) InvalidateCache() {
	s.videoCache.Flush()
}

// naturalLess compares two strings treating embedded numeric sequences as numbers,
// so that e.g. "file2" sorts before "file10".
func naturalLess(s1, s2 string) bool {
	i, j := 0, 0
	for i < len(s1) && j < len(s2) {
		for i < len(s1) && unicode.IsSpace(rune(s1[i])) {
			i++
		}
		for j < len(s2) && unicode.IsSpace(rune(s2[j])) {
			j++
		}

		if i >= len(s1) || j >= len(s2) {
			break
		}

		if unicode.IsDigit(rune(s1[i])) && unicode.IsDigit(rune(s2[j])) {
			var num1, num2 string
			for i < len(s1) && unicode.IsDigit(rune(s1[i])) {
				num1 += string(s1[i])
				i++
			}
			for j < len(s2) && unicode.IsDigit(rune(s2[j])) {
				num2 += string(s2[j])
				j++
			}

			n1, _ := strconv.Atoi(num1)
			n2, _ := strconv.Atoi(num2)
			if n1 != n2 {
				return n1 < n2
			}
		} else {
			if s1[i] != s2[j] {
				return s1[i] < s2[j]
			}
			i++
			j++
		}
	}

	return len(s1) < len(s2)
}
