package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"video-gallery/internal/domain/gallery"
)

const (
	// videoCacheTTL is how long a bucket listing is reused before re-listing
	videoCacheTTL = 5 * time.Minute
	// signedURLExpiry is the lifetime of the media URLs handed to browsers
	signedURLExpiry = 24 * time.Hour
)

// GalleryService handles gallery and video retrieval with caching
type GalleryService struct {
	repo      gallery.StorageRepository
	secretKey string

	mu          sync.Mutex
	videos      []gallery.Video
	videosUntil time.Time
}

// NewGalleryService creates a new GalleryService
func NewGalleryService(repo gallery.StorageRepository, secretKey string) *GalleryService {
	return &GalleryService{repo: repo, secretKey: secretKey}
}

// GetCategories returns all categories with their galleries, sorted by name
func (s *GalleryService) GetCategories() []gallery.Category {
	var categories []gallery.Category
	index := make(map[string]int)

	for _, g := range s.GetGalleries() {
		i, ok := index[g.Category]
		if !ok {
			i = len(categories)
			index[g.Category] = i
			categories = append(categories, gallery.Category{Name: g.Category, Stub: g.Category})
		}
		categories[i].Galleries = append(categories[i].Galleries, g)
	}

	sort.Slice(categories, func(i, j int) bool {
		return naturalLess(categories[i].Name, categories[j].Name)
	})
	return categories
}

// GetGallery returns a gallery by its stub
func (s *GalleryService) GetGallery(stub string) (gallery.Gallery, error) {
	for _, g := range s.GetGalleries() {
		if g.Stub == stub {
			return g, nil
		}
	}
	return gallery.Gallery{}, fmt.Errorf("gallery not found: %s", stub)
}

// GetGalleries returns all galleries with their videos, sorted by name.
// Galleries are keyed by name alone (matching the stub hash, which existing
// shared links depend on), so same-named galleries in different categories
// are merged under the first category seen.
func (s *GalleryService) GetGalleries() []gallery.Gallery {
	var galleries []gallery.Gallery
	index := make(map[string]int)

	for _, video := range s.GetVideos() {
		i, ok := index[video.Gallery]
		if !ok {
			i = len(galleries)
			index[video.Gallery] = i
			galleries = append(galleries, gallery.Gallery{
				Name:     video.Gallery,
				Category: video.Category,
				Stub:     "/gallery/" + s.galleryHash(video.Gallery),
			})
		}
		galleries[i].Videos = append(galleries[i].Videos, video)
	}

	sort.Slice(galleries, func(i, j int) bool {
		return naturalLess(galleries[i].Name, galleries[j].Name)
	})
	return galleries
}

// galleryHash derives a gallery's URL stub from its name salted with the secret
// key. It uses 16 base64url chars (~96 bits) so the stub stays short enough to
// share while being infeasible to guess by enumeration of the unauthenticated
// /gallery/ namespace.
func (s *GalleryService) galleryHash(galleryName string) string {
	sum := sha256.Sum256([]byte(galleryName + s.secretKey))
	return base64.URLEncoding.EncodeToString(sum[:])[:16]
}

// GetVideos returns all videos from storage, cached for videoCacheTTL
func (s *GalleryService) GetVideos() []gallery.Video {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.videos != nil && time.Now().Before(s.videosUntil) {
		return s.videos
	}

	log.Println("Listing videos from storage")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objects, err := s.repo.ListObjects(ctx)
	if err != nil {
		log.Printf("Failed to list storage objects: %v", err)
		return []gallery.Video{}
	}

	s.videos = s.buildVideos(ctx, objects)
	s.videosUntil = time.Now().Add(videoCacheTTL)
	return s.videos
}

// buildVideos pairs each video object with its same-basename thumbnail image
// and signs URLs for both.
func (s *GalleryService) buildVideos(ctx context.Context, objects []gallery.StorageObject) []gallery.Video {
	videosByBase := make(map[string]*gallery.Video)

	for _, obj := range objects {
		p, ok := gallery.ParseObjectPath(obj.Name)
		if !ok {
			continue
		}
		isVideo, isImage := gallery.IsVideo(p.File), gallery.IsImage(p.File)
		if !isVideo && !isImage {
			continue
		}

		signedURL, err := s.repo.GetSignedURL(ctx, obj.Name, signedURLExpiry)
		if err != nil {
			log.Printf("Error creating signed URL for %s: %v", obj.Name, err)
			continue
		}

		base := gallery.StripExt(obj.Name)
		video, ok := videosByBase[base]
		if !ok {
			video = &gallery.Video{
				Name:     gallery.StripExt(p.File),
				Category: p.Category,
				Gallery:  p.Gallery,
			}
			videosByBase[base] = video
		}

		if isVideo {
			video.Url = signedURL
			video.VideoPath = obj.Name
		} else {
			video.Thumbnail = &signedURL
			video.ThumbnailPath = obj.Name
		}
	}

	videos := make([]gallery.Video, 0, len(videosByBase))
	for _, video := range videosByBase {
		videos = append(videos, *video)
	}
	sort.Slice(videos, func(i, j int) bool {
		return naturalLess(videos[i].Name, videos[j].Name)
	})
	return videos
}

// InvalidateCache clears the video cache so subsequent reads fetch fresh data
func (s *GalleryService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos = nil
}

// naturalLess compares two strings treating embedded digit runs as numbers,
// so that e.g. "file2" sorts before "file10". Whitespace is ignored.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for {
		for i < len(a) && unicode.IsSpace(rune(a[i])) {
			i++
		}
		for j < len(b) && unicode.IsSpace(rune(b[j])) {
			j++
		}
		if i >= len(a) || j >= len(b) {
			return len(a)-i < len(b)-j
		}

		if isDigit(a[i]) && isDigit(b[j]) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			// Compare digit runs numerically without parsing, so arbitrarily
			// long runs cannot overflow: fewer significant digits is smaller.
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}

		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
