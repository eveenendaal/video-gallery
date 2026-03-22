package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)
	http.HandleFunc("/gallery/", galleryHandlers.PageHandler)
	http.HandleFunc(fmt.Sprintf("/%s/index", cfg.SecretKey), galleryHandlers.IndexHandler)
	http.HandleFunc(fmt.Sprintf("/%s/feed", cfg.SecretKey), galleryHandlers.FeedHandler)

	// Admin routes — all protected by the secret key
	http.HandleFunc(fmt.Sprintf("/%s/admin", cfg.SecretKey), adminHandlers.AdminHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/generate-thumbnail", cfg.SecretKey), adminHandlers.GenerateThumbnailHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/clear-thumbnail", cfg.SecretKey), adminHandlers.ClearThumbnailHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/bulk-generate-thumbnails", cfg.SecretKey), adminHandlers.BulkGenerateThumbnailsHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/bulk-clear-thumbnails", cfg.SecretKey), adminHandlers.BulkClearThumbnailsHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/fetch-movie-poster", cfg.SecretKey), adminHandlers.FetchMoviePosterHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/search-movie-poster", cfg.SecretKey), adminHandlers.SearchMoviePosterHandler)

	cfg.PrintServerStartMessage()
	return http.ListenAndServe(cfg.ServerAddress(), nil)
}
