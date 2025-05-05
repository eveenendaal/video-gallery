package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/eknkc/pug"

	"video-gallery/pkg/models"
	"video-gallery/pkg/services"
)

// GalleryHandler handles requests for the gallery index page
func GalleryHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Index")

	template, err := pug.CompileFile("./views/index.pug", pug.Options{})
	if err != nil {
		panic(err)
	}

	err = template.Execute(w, models.Index{
		Categories: services.GetCategories(),
	})

	if err != nil {
		panic(err)
	}
}

// FeedHandler handles requests for the gallery feed (JSON)
func FeedHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Feed")

	galleries := services.GetGalleries()

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

// PageHandler handles requests for individual gallery pages
func PageHandler(w http.ResponseWriter, r *http.Request) {
	// Get the path
	path := r.URL.String()

	gallery, err := services.GetGallery(path)
	if err != nil {
		log.Println("Gallery not found: " + path)
		http.NotFound(w, r)
		return
	}
	log.Println("Generating Gallery Page: " + path)

	template, err := pug.CompileFile("./views/gallery.pug", pug.Options{})
	if err != nil {
		panic(err)
	}

	err = template.Execute(w, gallery)
	if err != nil {
		panic(err)
	}
}
