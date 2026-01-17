package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/eknkc/pug"

	"video-gallery/pkg/models"
	"video-gallery/pkg/services"
)

// AdminHandler handles requests for the admin page
func AdminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Generating Admin Page")

	// Extract secret key from the URL path (format: /{SECRET_KEY}/admin)
	path := r.URL.Path
	secretKey := ""
	if len(path) > 1 {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) > 0 {
			secretKey = parts[0]
		}
	}

	template, err := pug.CompileFile("./assets/templates/admin.pug", pug.Options{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	adminData := models.Admin{
		Categories: services.GetCategories(),
		SecretKey:  secretKey,
	}
	log.Printf("Admin SecretKey: %s", adminData.SecretKey)
	err = template.Execute(w, adminData)

	if err != nil {
		http.Error(w, fmt.Sprintf("Template execution error: %v", err), http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}

// GenerateThumbnailHandler handles API requests to generate a single thumbnail with SSE progress
func GenerateThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	var videoPath string
	var timeMs int

	// Support both POST (JSON body) and GET (query params for EventSource)
	if r.Method == http.MethodGet {
		videoPath = r.URL.Query().Get("videoPath")
		timeMs = 1000
		if timeMsStr := r.URL.Query().Get("timeMs"); timeMsStr != "" {
			fmt.Sscanf(timeMsStr, "%d", &timeMs)
		}
	} else if r.Method == http.MethodPost {
		var req struct {
			VideoPath string `json:"videoPath"`
			TimeMs    int    `json:"timeMs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		videoPath = req.VideoPath
		timeMs = req.TimeMs
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if videoPath == "" {
		http.Error(w, "videoPath is required", http.StatusBadRequest)
		return
	}

	log.Printf("Generating thumbnail for video: %s at time: %dms", videoPath, timeMs)

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Progress callback
	progressCb := func(step string, progress int) {
		data := map[string]interface{}{
			"step":     step,
			"progress": progress,
		}
		jsonData, _ := json.Marshal(data)
		w.Write([]byte("data: "))
		w.Write(jsonData)
		w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	// Generate thumbnail with progress updates
	if err := services.GenerateThumbnailWithProgress(videoPath, timeMs, progressCb); err != nil {
		log.Printf("Error generating thumbnail: %v", err)
		errorData := map[string]interface{}{
			"error":    err.Error(),
			"progress": -1,
		}
		jsonData, _ := json.Marshal(errorData)
		w.Write([]byte("data: "))
		w.Write(jsonData)
		w.Write([]byte("\n\n"))
		flusher.Flush()
		return
	}
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
