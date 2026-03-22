package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/eknkc/pug"

	"video-gallery/internal/application"
	"video-gallery/internal/domain/gallery"
)

// Index is the view model for the gallery index page
type Index struct {
	Categories []gallery.Category
}

// GalleryHandlers holds the HTTP handlers for the public gallery routes
type GalleryHandlers struct {
	galleryService *application.GalleryService
}

// NewGalleryHandlers creates GalleryHandlers with an injected GalleryService
func NewGalleryHandlers(svc *application.GalleryService) *GalleryHandlers {
	return &GalleryHandlers{galleryService: svc}
}

// IndexHandler handles requests for the gallery index page
func (h *GalleryHandlers) IndexHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Index")

	template, err := pug.CompileFile("./assets/templates/index.pug", pug.Options{})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	if err = template.Execute(w, Index{Categories: h.galleryService.GetCategories()}); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}

// FeedHandler handles requests for the gallery feed (JSON)
func (h *GalleryHandlers) FeedHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Feed")

	galleries := h.galleryService.GetGalleries()

	jsonData, err := json.Marshal(galleries)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("JSON marshaling error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(jsonData); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// PageHandler handles requests for individual gallery pages
func (h *GalleryHandlers) PageHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.String()

	g, err := h.galleryService.GetGallery(path)
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

	if err = template.Execute(w, g); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}
