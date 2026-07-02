package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/spf13/cobra"

	"video-gallery/internal/application"
	gcsrepo "video-gallery/internal/infrastructure/gcs"
	"video-gallery/internal/infrastructure/ffmpeg"
	"video-gallery/internal/infrastructure/tmdb"
	"video-gallery/pkg/config"
	"video-gallery/pkg/handlers"
)

// newServeCmd creates a new command for serving the web application
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the web server",
		Long:  `Start the web server to serve the gallery content via HTTP.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}
			if err := serveWebsite(cfg); err != nil {
				log.Printf("Server error: %v", err)
				os.Exit(1)
			}
		},
	}
}

// serveWebsite wires all dependencies and starts the HTTP server
func serveWebsite(cfg *config.Config) error {
	// --- Infrastructure ---
	ctx := context.Background()
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %v", err)
	}

	storageRepo := gcsrepo.NewStorageRepository(cfg.BucketName, gcsClient)
	videoProcessor := ffmpeg.NewProcessor()
	posterClient := tmdb.NewClient(cfg.TMDbAPIKey)

	// --- Application services ---
	gallerySvc := application.NewGalleryService(storageRepo, cfg.SecretKey)
	thumbnailSvc := application.NewThumbnailService(storageRepo, videoProcessor, gallerySvc)
	posterSvc := application.NewPosterService(storageRepo, posterClient, gallerySvc)

	// --- Presentation ---
	galleryHandlers := handlers.NewGalleryHandlers(gallerySvc)
	adminHandlers := handlers.NewAdminHandlers(gallerySvc, thumbnailSvc, posterSvc, cfg.SecretKey)

	// --- Routes ---
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir("./public"))
	mux.Handle("/", fileServer)
	mux.HandleFunc("/gallery/", galleryHandlers.PageHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/index", cfg.SecretKey), galleryHandlers.IndexHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/feed", cfg.SecretKey), galleryHandlers.FeedHandler)

	// Admin routes — all protected by the secret key
	mux.HandleFunc(fmt.Sprintf("/%s/admin", cfg.SecretKey), adminHandlers.AdminHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/admin/api/generate-thumbnail", cfg.SecretKey), adminHandlers.GenerateThumbnailHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/admin/api/clear-thumbnail", cfg.SecretKey), adminHandlers.ClearThumbnailHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/admin/api/bulk-generate-thumbnails", cfg.SecretKey), adminHandlers.BulkGenerateThumbnailsHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/admin/api/bulk-clear-thumbnails", cfg.SecretKey), adminHandlers.BulkClearThumbnailsHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/admin/api/fetch-movie-poster", cfg.SecretKey), adminHandlers.FetchMoviePosterHandler)
	mux.HandleFunc(fmt.Sprintf("/%s/admin/api/search-movie-poster", cfg.SecretKey), adminHandlers.SearchMoviePosterHandler)

	// No WriteTimeout: the admin SSE endpoints hold long-lived streaming responses.
	server := &http.Server{
		Addr:              cfg.ServerAddress(),
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	cfg.PrintServerStartMessage()
	return server.ListenAndServe()
}

// securityHeaders adds standard browser security headers to every response.
// Referrer-Policy is set to no-referrer because the secret key is part of the
// URL and must never leak through the Referer header. script-src/style-src
// allow 'unsafe-inline' because the pug templates use inline scripts and styles.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' https: data:; media-src 'self' https:; "+
				"script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; "+
				"connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}
