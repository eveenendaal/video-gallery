package services

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"video-gallery/pkg/config"

	"cloud.google.com/go/storage"
	"github.com/patrickmn/go-cache"
	"google.golang.org/api/iterator"

	"video-gallery/pkg/models"
)

var videoCache = cache.New(5*time.Minute, 10*time.Minute)
var appConfig *config.Config

// InitService initializes the service with the given configuration
func InitService(cfg *config.Config) {
	appConfig = cfg
}

// GetCategories returns all categories with their galleries
func GetCategories() []models.Category {
	var categories []models.Category
	for _, gallery := range GetGalleries() {
		category := gallery.Category
		// Check if a category already exists
		exists := false
		for i, c := range categories {
			if c.Name == category {
				categories[i].Galleries = append(categories[i].Galleries, gallery)
				exists = true
				break
			}
		}
		if !exists {
			categories = append(categories, models.Category{
				Name:      category,
				Stub:      category,
				Galleries: []models.Gallery{gallery},
			})
		}
	}
	return categories
}

// GetGallery returns a gallery by its stub
func GetGallery(stub string) (models.Gallery, error) {
	// Get gallery
	for _, gallery := range GetGalleries() {
		if gallery.Stub == stub {
			return gallery, nil
		}
	}
	return models.Gallery{}, fmt.Errorf("gallery not found")
}

// GetGalleries returns all galleries with their videos
func GetGalleries() []models.Gallery {
	videos := GetVideos()
	secretKey := appConfig.GetSecretKey()

	var galleries []models.Gallery
	for _, video := range videos {
		category := video.Category
		gallery := video.Gallery
		// Check if gallery already exists
		exists := false
		for i, g := range galleries {
			if g.Name == gallery {
				galleries[i].Videos = append(galleries[i].Videos, video)
				exists = true
				break
			}
		}
		if !exists {
			// Generate Hash
			hash := sha1.New()
			hash.Write([]byte(gallery + secretKey))
			secretKey := base64.URLEncoding.EncodeToString(hash.Sum(nil))[0:4]

			galleries = append(galleries, models.Gallery{
				Name:     gallery,
				Category: category,
				Stub:     "/gallery/" + secretKey,
				Videos:   []models.Video{video},
			})
		}
	}
	return galleries
}

// GetVideos returns all videos from the storage bucket
func GetVideos() []models.Video {
	// Check if Videos are cached
	if cachedVideos, found := videoCache.Get("videos"); found {
		log.Println("Using Cached Videos")
		return cachedVideos.([]models.Video)
	}
	log.Println("Getting Videos")

	// Get bucket name from config
	bucketName := appConfig.GetBucketName()

	// Initialize Cloud Storage
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	bucket := storageClient.Bucket(bucketName)
	files := bucket.Objects(context.Background(), nil)
	videosMap := make(map[string]models.Video)

	// Allowed Extensions
	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}
	extensionRegex, _ := regexp.Compile(`\.[a-zA-Z0-9]+$`)

	// Iterate through videos
	for {
		file, err := files.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		parts := strings.Split(file.Name, "/")
		if len(parts) == 3 && parts[2] != "" {
			category := parts[0]
			gallery := parts[1]
			filename := parts[2]
			// Create a Signed 24-Hour URL
			signedUrl, err := bucket.SignedURL(file.Name, &storage.SignedURLOptions{
				Expires: time.Now().Add(24 * time.Hour),
				Method:  "GET",
			})
			if err != nil {
				log.Fatal(err)
			}
			// Remove extension from the filename
			fileBase := extensionRegex.ReplaceAll([]byte(filename), []byte(""))

			// If Video doesn't exist
			if _, ok := videosMap[string(fileBase)]; !ok {
				videosMap[string(fileBase)] = models.Video{
					Name:     string(fileBase),
					Category: category,
					Gallery:  gallery,
				}
			}

			// Check if video already exists
			if video, ok := videosMap[string(fileBase)]; ok {
				for _, extension := range videoExtensions {
					if strings.HasSuffix(filename, extension) {
						videosMap[string(fileBase)] = models.Video{
							Name:      video.Name,
							Category:  video.Category,
							Gallery:   video.Gallery,
							Url:       signedUrl,
							Thumbnail: video.Thumbnail,
						}
					}
				}
				for _, extension := range imageExtensions {
					if strings.HasSuffix(filename, extension) {
						videosMap[string(fileBase)] = models.Video{
							Name:      video.Name,
							Category:  video.Category,
							Gallery:   video.Gallery,
							Url:       video.Url,
							Thumbnail: &signedUrl,
						}
					}
				}
			}
		}
	}
	// Convert Map to Array
	var videos []models.Video
	for _, video := range videosMap {
		videos = append(videos, video)
	}

	// Sort videos alphabetically by name
	sort.Slice(videos, func(i, j int) bool {
		return videos[i].Name < videos[j].Name
	})

	// Cache Videos
	videoCache.Set("videos", videos, cache.DefaultExpiration)
	return videos
}
