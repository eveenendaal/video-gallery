package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/eknkc/pug"

	"video-gallery/pkg/models"
	"video-gallery/pkg/services"
)

// AdminHandler handles requests for the admin page
func AdminHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Admin Page")

	template, err := pug.CompileFile("./assets/templates/admin.pug", pug.Options{})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	err = template.Execute(w, models.Admin{
		Categories: services.GetCategories(),
	})

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}

// GenerateThumbnailHandler handles API requests to generate a single thumbnail
func GenerateThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VideoPath string `json:"videoPath"`
		TimeMs    int    `json:"timeMs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Generating thumbnail for video: %s at time: %dms", req.VideoPath, req.TimeMs)

	if err := services.GenerateThumbnail(req.VideoPath, req.TimeMs); err != nil {
		log.Printf("Error generating thumbnail: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Thumbnail generated successfully",
	})
}

// ClearThumbnailHandler handles API requests to clear a single thumbnail
func ClearThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ThumbnailPath string `json:"thumbnailPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Clearing thumbnail: %s", req.ThumbnailPath)

	if err := services.ClearThumbnail(req.ThumbnailPath); err != nil {
		log.Printf("Error clearing thumbnail: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Thumbnail cleared successfully",
	})
}

// BulkGenerateThumbnailsHandler handles API requests to generate all thumbnails
func BulkGenerateThumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TimeMs int  `json:"timeMs"`
		Force  bool `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Bulk generating thumbnails at time: %dms, force: %v", req.TimeMs, req.Force)

	processed, errors, err := services.BulkGenerateThumbnails(req.TimeMs, req.Force)
	if err != nil {
		log.Printf("Error in bulk generate: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Bulk thumbnail generation completed",
		"processed": processed,
		"errors":    errors,
	})
}

// BulkClearThumbnailsHandler handles API requests to clear all thumbnails
func BulkClearThumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Bulk clearing thumbnails")

	deleted, err := services.BulkClearThumbnails()
	if err != nil {
		log.Printf("Error in bulk clear: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "All thumbnails cleared successfully",
		"deleted": deleted,
	})
}
