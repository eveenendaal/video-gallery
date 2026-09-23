package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"video-gallery/internal/application"
	"video-gallery/internal/domain/gallery"
)

// maxRequestBodyBytes caps JSON request bodies on the admin API endpoints.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// defaultThumbnailTimeMs is the frame offset used when a request omits timeMs.
const defaultThumbnailTimeMs = 1000

// isValidObjectPath checks that a caller-supplied storage path has the expected
// "category/gallery/file" shape and an extension accepted by isAllowed. This
// keeps the admin API from being used to read or delete arbitrary bucket objects.
func isValidObjectPath(path string, isAllowed func(string) bool) bool {
	if strings.Contains(path, "..") || strings.Contains(path, "?") {
		return false
	}
	p, ok := gallery.ParseObjectPath(path)
	return ok && isAllowed(p.File)
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
func (h *AdminHandlers) AdminHandler(w http.ResponseWriter, _ *http.Request) {
	log.Println("Generating Admin Page")
	renderTemplate(w, "admin.pug", Admin{
		Categories: h.galleryService.GetCategories(),
		SecretKey:  h.secretKey,
	})
}

// GenerateThumbnailHandler streams SSE progress while generating a single
// thumbnail. It is a GET endpoint because the admin page drives it with EventSource.
func (h *AdminHandlers) GenerateThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	videoPath := query.Get("videoPath")
	timeMs := defaultThumbnailTimeMs
	if s := query.Get("timeMs"); s != "" {
		parsed, err := strconv.Atoi(s)
		if err != nil || parsed < 0 {
			http.Error(w, "timeMs must be a non-negative integer", http.StatusBadRequest)
			return
		}
		timeMs = parsed
	}
	if !isValidObjectPath(videoPath, gallery.IsVideo) {
		http.Error(w, "videoPath must be a valid video object path", http.StatusBadRequest)
		return
	}

	log.Printf("Generating thumbnail for video: %s at time: %dms", videoPath, timeMs)
	streamSSE(w, func(progressCb application.ProgressCallback) error {
		return h.thumbnailService.GenerateThumbnail(videoPath, timeMs, progressCb)
	})
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
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if !isValidObjectPath(req.ThumbnailPath, gallery.IsImage) {
		http.Error(w, "thumbnailPath must be a valid image object path", http.StatusBadRequest)
		return
	}

	log.Printf("Clearing thumbnail: %s", req.ThumbnailPath)

	if err := h.thumbnailService.ClearThumbnail(req.ThumbnailPath); err != nil {
		log.Printf("Error clearing thumbnail: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"message": "Thumbnail cleared successfully"})
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
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if req.TimeMs < 0 {
		http.Error(w, "timeMs must not be negative", http.StatusBadRequest)
		return
	}

	log.Printf("Bulk generating thumbnails at time: %dms, force: %v", req.TimeMs, req.Force)

	processed, failed, err := h.thumbnailService.BulkGenerateThumbnails(req.TimeMs, req.Force)
	if err != nil {
		log.Printf("Error in bulk generate: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"message":   "Bulk thumbnail generation completed",
		"processed": processed,
		"errors":    failed,
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

	writeJSON(w, map[string]any{
		"message": "All thumbnails cleared successfully",
		"deleted": deleted,
	})
}

// FetchMoviePosterHandler streams SSE progress while fetching a movie poster.
// It is a GET endpoint because the admin page drives it with EventSource.
func (h *AdminHandlers) FetchMoviePosterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	videoPath := query.Get("videoPath")
	movieTitle := query.Get("movieTitle")
	movieID := 0
	if s := query.Get("movieId"); s != "" {
		parsed, err := strconv.Atoi(s)
		if err != nil || parsed < 0 {
			http.Error(w, "movieId must be a non-negative integer", http.StatusBadRequest)
			return
		}
		movieID = parsed
	}
	if movieTitle == "" {
		http.Error(w, "movieTitle is required", http.StatusBadRequest)
		return
	}
	if !isValidObjectPath(videoPath, gallery.IsVideo) {
		http.Error(w, "videoPath must be a valid video object path", http.StatusBadRequest)
		return
	}

	log.Printf("Fetching movie poster for: %s (video: %s)", movieTitle, videoPath)
	streamSSE(w, func(progressCb application.ProgressCallback) error {
		return h.posterService.FetchMoviePoster(videoPath, movieTitle, movieID, progressCb)
	})
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

	writeJSON(w, results)
}

// decodeJSONBody decodes a size-limited JSON request body into v, writing a
// 400 response and returning false on failure.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

// streamSSE runs op, streaming each progress update as a server-sent event
// and, if op fails, a final {"error", "progress": -1} event.
func streamSSE(w http.ResponseWriter, op func(application.ProgressCallback) error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func(data map[string]any) {
		jsonData, _ := json.Marshal(data)
		w.Write([]byte("data: " + string(jsonData) + "\n\n"))
		flusher.Flush()
	}

	if err := op(func(step string, progress int) {
		send(map[string]any{"step": step, "progress": progress})
	}); err != nil {
		log.Printf("Admin operation failed: %v", err)
		send(map[string]any{"error": err.Error(), "progress": -1})
	}
}
