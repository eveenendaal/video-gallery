package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eknkc/pug"

	"video-gallery/internal/application"
	"video-gallery/internal/domain/gallery"
)

// maxRequestBodyBytes caps JSON request bodies on the admin API endpoints.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

var (
	videoExtensions = []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions = []string{".jpg", ".jpeg", ".png"}
)

// isValidObjectPath checks that a caller-supplied storage path has the expected
// "category/gallery/file" shape and one of the allowed extensions. This keeps
// the admin API from being used to read or delete arbitrary bucket objects.
func isValidObjectPath(path string, allowedExts []string) bool {
	if path == "" || strings.Contains(path, "..") || strings.Contains(path, "?") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(parts[2]))
	for _, allowed := range allowedExts {
		if ext == allowed {
			return true
		}
	}
	return false
}

// Admin is the view model for the admin page
type Admin struct {
	Categories []gallery.Category
	SecretKey  string
}

// AdminHandlers holds the HTTP handlers for the admin routes
type AdminHandlers struct {
	galleryService   *application.GalleryService
	thumbnailService *application.ThumbnailService
	posterService    *application.PosterService
	secretKey        string
}

// NewAdminHandlers creates AdminHandlers with injected application services
func NewAdminHandlers(
	gallerySvc *application.GalleryService,
	thumbnailSvc *application.ThumbnailService,
	posterSvc *application.PosterService,
	secretKey string,
) *AdminHandlers {
	return &AdminHandlers{
		galleryService:   gallerySvc,
		thumbnailService: thumbnailSvc,
		posterService:    posterSvc,
		secretKey:        secretKey,
	}
}

// AdminHandler handles requests for the admin page
func (h *AdminHandlers) AdminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Generating Admin Page")

	// Extract secret key from URL path (format: /{SECRET_KEY}/admin)
	secretKey := ""
	path := r.URL.Path
	if len(path) > 1 {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) > 0 {
			secretKey = parts[0]
		}
	}

	template, err := pug.CompileFile("./assets/templates/admin.pug", pug.Options{})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	if err = template.Execute(w, Admin{
		Categories: h.galleryService.GetCategories(),
		SecretKey:  secretKey,
	}); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
		return
	}
}

// GenerateThumbnailHandler handles API requests to generate a single thumbnail with SSE progress
func (h *AdminHandlers) GenerateThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	var videoPath string
	var timeMs int

	// Support both POST (JSON body) and GET (query params for EventSource)
	if r.Method == http.MethodGet {
		videoPath = r.URL.Query().Get("videoPath")
		timeMs = 1000
		if timeMsStr := r.URL.Query().Get("timeMs"); timeMsStr != "" {
			if parsed, err := strconv.Atoi(timeMsStr); err == nil {
				timeMs = parsed
			}
		}
	} else if r.Method == http.MethodPost {
		var req struct {
			VideoPath string `json:"videoPath"`
			TimeMs    int    `json:"timeMs"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
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

	if !isValidObjectPath(videoPath, videoExtensions) {
		http.Error(w, "videoPath must be a valid video object path", http.StatusBadRequest)
		return
	}
	if timeMs < 0 {
		http.Error(w, "timeMs must not be negative", http.StatusBadRequest)
		return
	}

	log.Printf("Generating thumbnail for video: %s at time: %dms", videoPath, timeMs)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	progressCb := makeSSEProgressCallback(w, flusher)

	if err := h.thumbnailService.GenerateThumbnail(videoPath, timeMs, progressCb); err != nil {
		log.Printf("Error generating thumbnail: %v", err)
		sendSSEError(w, flusher, err.Error())
	}
}

// ClearThumbnailHandler handles API requests to clear a single thumbnail
func (h *AdminHandlers) ClearThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ThumbnailPath string `json:"thumbnailPath"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidObjectPath(req.ThumbnailPath, imageExtensions) {
		http.Error(w, "thumbnailPath must be a valid image object path", http.StatusBadRequest)
		return
	}

	log.Printf("Clearing thumbnail: %s", req.ThumbnailPath)

	if err := h.thumbnailService.ClearThumbnail(req.ThumbnailPath); err != nil {
		log.Printf("Error clearing thumbnail: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Thumbnail cleared successfully"})
}

// BulkGenerateThumbnailsHandler handles API requests to generate all thumbnails
func (h *AdminHandlers) BulkGenerateThumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TimeMs int  `json:"timeMs"`
		Force  bool `json:"force"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TimeMs < 0 {
		http.Error(w, "timeMs must not be negative", http.StatusBadRequest)
		return
	}

	log.Printf("Bulk generating thumbnails at time: %dms, force: %v", req.TimeMs, req.Force)

	processed, errors, err := h.thumbnailService.BulkGenerateThumbnails(req.TimeMs, req.Force)
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
func (h *AdminHandlers) BulkClearThumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Bulk clearing thumbnails")

	deleted, err := h.thumbnailService.BulkClearThumbnails()
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

// FetchMoviePosterHandler handles API requests to fetch a movie poster with SSE progress
func (h *AdminHandlers) FetchMoviePosterHandler(w http.ResponseWriter, r *http.Request) {
	var videoPath, movieTitle string
	var movieID int

	// Support both POST (JSON body) and GET (query params for EventSource)
	if r.Method == http.MethodGet {
		videoPath = r.URL.Query().Get("videoPath")
		movieTitle = r.URL.Query().Get("movieTitle")
		if movieIDStr := r.URL.Query().Get("movieId"); movieIDStr != "" {
			parsed, err := strconv.Atoi(movieIDStr)
			if err != nil || parsed < 0 {
				http.Error(w, "movieId must be a non-negative integer", http.StatusBadRequest)
				return
			}
			movieID = parsed
		}
	} else if r.Method == http.MethodPost {
		var req struct {
			VideoPath  string `json:"videoPath"`
			MovieTitle string `json:"movieTitle"`
			MovieID    int    `json:"movieId"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		videoPath = req.VideoPath
		movieTitle = req.MovieTitle
		movieID = req.MovieID
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if movieTitle == "" {
		http.Error(w, "movieTitle is required", http.StatusBadRequest)
		return
	}
	if !isValidObjectPath(videoPath, videoExtensions) {
		http.Error(w, "videoPath must be a valid video object path", http.StatusBadRequest)
		return
	}

	log.Printf("Fetching movie poster for: %s (video: %s)", movieTitle, videoPath)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	progressCb := makeSSEProgressCallback(w, flusher)

	if err := h.posterService.FetchMoviePoster(videoPath, movieTitle, movieID, progressCb); err != nil {
		log.Printf("Error fetching movie poster: %v", err)
		sendSSEError(w, flusher, err.Error())
	}
}

// SearchMoviePosterHandler handles API requests to search for movie posters
func (h *AdminHandlers) SearchMoviePosterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	movieTitle := r.URL.Query().Get("movieTitle")
	if movieTitle == "" {
		http.Error(w, "movieTitle query parameter is required", http.StatusBadRequest)
		return
	}

	log.Printf("Searching movie posters for: %s", movieTitle)

	results, err := h.posterService.SearchMoviePoster(movieTitle)
	if err != nil {
		log.Printf("Error searching movie posters: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// makeSSEProgressCallback returns a ProgressCallback that streams JSON progress events
func makeSSEProgressCallback(w http.ResponseWriter, flusher http.Flusher) application.ProgressCallback {
	return func(step string, progress int) {
		data := map[string]interface{}{"step": step, "progress": progress}
		jsonData, _ := json.Marshal(data)
		w.Write([]byte("data: "))
		w.Write(jsonData)
		w.Write([]byte("\n\n"))
		flusher.Flush()
	}
}

// sendSSEError streams an SSE error event
func sendSSEError(w http.ResponseWriter, flusher http.Flusher, errMsg string) {
	data := map[string]interface{}{"error": errMsg, "progress": -1}
	jsonData, _ := json.Marshal(data)
	w.Write([]byte("data: "))
	w.Write(jsonData)
	w.Write([]byte("\n\n"))
	flusher.Flush()
}
