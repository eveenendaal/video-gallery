package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"video-gallery/pkg/config"
	"video-gallery/pkg/handlers"
	"video-gallery/pkg/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize services
	services.InitService(cfg)

	// Set up HTTP handlers
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)
	http.HandleFunc("/gallery/", handlers.PageHandler)
	http.HandleFunc(fmt.Sprintf("/%s/index", cfg.SecretKey), handlers.GalleryHandler)
	http.HandleFunc(fmt.Sprintf("/%s/feed", cfg.SecretKey), handlers.FeedHandler)

	// Start server
	cfg.PrintServerStartMessage()
	if err := http.ListenAndServe(cfg.ServerAddress(), nil); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
