package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"

	"github.com/eknkc/pug"

	"video-gallery/internal/application"
	"video-gallery/internal/domain/gallery"
)

// templateDir holds the pug templates, relative to the working directory
const templateDir = "./assets/templates"

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
	renderTemplate(w, "index.pug", Index{Categories: h.galleryService.GetCategories()})
}

// FeedHandler handles requests for the gallery feed (JSON)
func (h *GalleryHandlers) FeedHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Feed")
	writeJSON(w, h.galleryService.GetGalleries())
}

// PageHandler handles requests for individual gallery pages
func (h *GalleryHandlers) PageHandler(w http.ResponseWriter, r *http.Request) {
	g, err := h.galleryService.GetGallery(r.URL.Path)
	if err != nil {
		log.Printf("Gallery not found: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	log.Printf("Generating Gallery Page: %s", r.URL.Path)
	renderTemplate(w, "gallery.pug", g)
}

// renderTemplate compiles and executes the named pug template. Templates are
// compiled per request so edits (and the inlined stylesheet) take effect
// without a restart.
func renderTemplate(w http.ResponseWriter, name string, data any) {
	template, err := pug.CompileFile(filepath.Join(templateDir, name), pug.Options{})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}
	if err := template.Execute(w, data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
	}
}

// writeJSON encodes v as a JSON response body
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}
