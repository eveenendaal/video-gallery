package main

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/eknkc/pug"
	"google.golang.org/api/iterator"
)

type Category struct {
	Name      string    `json:"name"`
	Stub      string    `json:"stub"`
	Galleries []Gallery `json:"galleries"`
}

type Gallery struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Stub     string  `json:"stub"`
	Videos   []Video `json:"videos"`
}

type Video struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	Gallery   string `json:"gallery"`
	Url       string `json:"url"`
	Thumbnail string `json:"thumbnail"`
}

type Index struct {
	Categories []Category
}

func getCategories() []Category {
	var categories []Category
	for _, gallery := range getGalleries() {
		category := gallery.Category
		// Check if category already exists
		exists := false
		for i, c := range categories {
			if c.Name == category {
				categories[i].Galleries = append(categories[i].Galleries, gallery)
				exists = true
				break
			}
		}
		if !exists {
			categories = append(categories, Category{
				Name:      category,
				Stub:      category,
				Galleries: []Gallery{gallery},
			})
		}
	}
	return categories
}

func getGallery(stub string) (Gallery, error) {
	// Get gallery
	for _, gallery := range getGalleries() {
		if gallery.Stub == stub {
			return gallery, nil
		}
	}
	return Gallery{}, fmt.Errorf("gallery not found")
}

func getGalleries() []Gallery {
	videos := getVideos()
	var galleries []Gallery
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
			hash.Write([]byte(gallery))
			secretKey := base64.URLEncoding.EncodeToString(hash.Sum(nil))[0:4]

			galleries = append(galleries, Gallery{
				Name:     gallery,
				Category: category,
				Stub:     "/gallery/" + secretKey,
				Videos:   []Video{video},
			})
		}
	}
	return galleries
}

func getVideos() []Video {
	// Get Environment Variables
	// projectId := os.Getenv("GCLOUD_PROJECT")
	bucketName := os.Getenv("BUCKET_NAME")

	// Initialize Cloud Storage
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	bucket := storageClient.Bucket(bucketName)
	files := bucket.Objects(context.Background(), nil)
	var videos []Video

	// Allowed Extensions
	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}

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
			// Create Signed 24 Hour URL
			signedUrl, err := bucket.SignedURL(file.Name, &storage.SignedURLOptions{
				Expires: time.Now().Add(24 * time.Hour),
				Method:  "GET",
			})
			if err != nil {
				log.Fatal(err)
			}

			// Check if video
			for _, extension := range videoExtensions {
				name := strings.TrimRight(filename, extension)

				if strings.HasSuffix(filename, extension) {
					found := false
					for _, next := range videos {
						if next.Name == name {
							found = true
							next.Url = signedUrl
						}
					}
					if !found {
						videos = append(videos, Video{
							Name:      name,
							Category:  category,
							Gallery:   gallery,
							Url:       signedUrl,
							Thumbnail: "",
						})
					}
				}
			}

			for _, extension := range imageExtensions {
				name := strings.TrimRight(filename, extension)

				if strings.HasSuffix(filename, extension) {
					found := false
					for _, next := range videos {
						if next.Name == name {
							found = true
							next.Thumbnail = signedUrl
						}
					}
					if !found {
						videos = append(videos, Video{
							Name:      strings.TrimRight(filename, extension),
							Category:  category,
							Gallery:   gallery,
							Url:       "",
							Thumbnail: signedUrl,
						})
					}
				}
			}
		}
	}
	return videos
}

func galleryHandler(w http.ResponseWriter, r *http.Request) {
	template, err := pug.CompileFile("./views/index.pug", pug.Options{})
	if err != nil {
		panic(err)
	}

	err = template.Execute(w, Index{
		Categories: getCategories(),
	})

	if err != nil {
		panic(err)
	}
}

func feedHandler(w http.ResponseWriter, r *http.Request) {
	galleries := getGalleries()
	// Convert to JSON
	jsonString, err := json.Marshal(galleries)
	if err != nil {
		panic(err)
	}
	// Write JSON
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonString)
	if err != nil {
		return
	}
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	// Get path
	path := r.URL.String()
	gallery, err := getGallery(path)
	if err != nil {
		panic(err)
	}

	template, err := pug.CompileFile("./views/gallery.pug", pug.Options{})
	if err != nil {
		panic(err)
	}

	err = template.Execute(w, gallery)
	if err != nil {
		panic(err)
	}
}

func main() {
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		panic("SECRET_KEY not set")
	}
	// Service
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)
	http.HandleFunc("/gallery/", pageHandler)
	http.HandleFunc("/"+secretKey+"/index", galleryHandler)
	http.HandleFunc("/"+secretKey+"/feed", feedHandler)

	// Read Environment Variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting server at port " + port + "\n")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}