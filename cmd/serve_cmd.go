package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"video-gallery/pkg/config"
	"video-gallery/pkg/handlers"
	"video-gallery/pkg/services"
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
			services.InitService(cfg)
			serveWebsite(cfg)
		},
	}
}

// serveWebsite runs the web server to serve the gallery content
func serveWebsite(cfg *config.Config) {
	// Use the original web server functionality
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)
	http.HandleFunc("/gallery/", handlers.PageHandler)
	http.HandleFunc(fmt.Sprintf("/%s/index", cfg.SecretKey), handlers.GalleryHandler)
	http.HandleFunc(fmt.Sprintf("/%s/feed", cfg.SecretKey), handlers.FeedHandler)
	
	// Admin routes - all protected by secret key
	http.HandleFunc(fmt.Sprintf("/%s/admin", cfg.SecretKey), handlers.AdminHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/generate-thumbnail", cfg.SecretKey), handlers.GenerateThumbnailHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/clear-thumbnail", cfg.SecretKey), handlers.ClearThumbnailHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/bulk-generate-thumbnails", cfg.SecretKey), handlers.BulkGenerateThumbnailsHandler)
	http.HandleFunc(fmt.Sprintf("/%s/admin/api/bulk-clear-thumbnails", cfg.SecretKey), handlers.BulkClearThumbnailsHandler)

	// Start server
	cfg.PrintServerStartMessage()
	if err := http.ListenAndServe(cfg.ServerAddress(), nil); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
