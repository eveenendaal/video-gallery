package main

import (
	"context"
	"encoding/json"
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
	Name      string
	Galleries []Gallery
}

type Gallery struct {
	Name     string
	Category string
	Stub     string
	Videos   []Video
}

type Video struct {
	Name      string
	Category  string
	Gallery   string
	Url       string
	Thumbnail string
}

type Index struct {
	Categories []Category
}

func getCategories() []Category {
	categories := []Category{}
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
				Galleries: []Gallery{gallery},
			})
		}
	}
	return categories
}

func getGallery(stub string) (Gallery, error) {
	for _, gallery := range getGalleries() {
		if gallery.Stub == strings.ToLower(stub) {
			return gallery, nil
		}
	}
	return Gallery{}, fmt.Errorf("Gallery not found")
}

func getGalleries() []Gallery {
	videos := getVideos()
	galleries := []Gallery{}
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
			galleries = append(galleries, Gallery{
				Name:     gallery,
				Category: category,
				Stub:     "/gallery/" + strings.ToLower(gallery),
				Videos:   []Video{video},
			})
		}
	}
	return galleries
}

func getVideos() []Video {
	// Get Environment Variables
	// projectId := os.Getenv("GCLOUD_PROJECT")
	bucketName := "veenendaal-videos-demo" // bucketName := os.Getenv("BUCKET_NAME")
	// secretKey := os.Getenv("SECRET_KEY")

	// Initialize Cloud Storage
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	bucket := storageClient.Bucket(bucketName)
	files := bucket.Objects(context.Background(), nil)
	videos := []Video{}

	// Allowed Extensions
	videoExtensions := []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions := []string{".jpg", ".jpeg", ".png"}

	// Iterate through videos
	for {
		file, err := files.Next()
		if err == iterator.Done {
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
					println("Video: " + filename)
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
					println("Image: " + filename)
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

func handlePage(w http.ResponseWriter, r *http.Request) {
	// Get path
	path := r.URL.String()
	println(path)
	gallery, err := getGallery(path)
	if err != nil {
		panic(err)
	}

	template, err := pug.CompileFile("./views/gallery.pug", pug.Options{})
	if err != nil {
		panic(err)
	}

	println(gallery.Name)
	err = template.Execute(w, gallery)

	if err != nil {
		panic(err)
	}
}

func main() {
	// Service
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)
	http.HandleFunc("/gallery", galleryHandler)
	http.HandleFunc("/gallery/", handlePage)
	http.HandleFunc("/feed", feedHandler)

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
