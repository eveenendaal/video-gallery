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

	template, err := pug.CompileFile("./assets/templates/index.pug", pug.Options{})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	err = template.Execute(w, models.Index{
		Categories: services.GetCategories(),
	})

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}

// FeedHandler handles requests for the gallery feed (JSON)
func FeedHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Feed")

	galleries := services.GetGalleries()

	// Convert to JSON
	jsonData, err := json.Marshal(galleries)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("JSON marshaling error: %v", err)
		return
	}

	// Write JSON
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(jsonData); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// PageHandler handles requests for individual gallery pages
func PageHandler(w http.ResponseWriter, r *http.Request) {
	// Get the path
	path := r.URL.String()

	gallery, err := services.GetGallery(path)
	if err != nil {
		log.Printf("Gallery not found: %s", path)
		http.NotFound(w, r)
		return
	}
	log.Printf("Generating Gallery Page: %s", path)

	template, err := pug.CompileFile("./assets/templates/gallery.pug", pug.Options{})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	if err = template.Execute(w, gallery); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}
