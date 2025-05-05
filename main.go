package main

import (
	"log"
	"net/http"

	"video-gallery/pkg/config"
	"video-gallery/pkg/handlers"
	"video-gallery/pkg/services"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize services
	services.InitService(cfg)

	// Set up HTTP handlers
	fileServer := http.FileServer(http.Dir("./public"))
	http.Handle("/", fileServer)
	http.HandleFunc("/gallery/", handlers.PageHandler)
	http.HandleFunc("/"+cfg.GetSecretKey()+"/index", handlers.GalleryHandler)
	http.HandleFunc("/"+cfg.GetSecretKey()+"/feed", handlers.FeedHandler)

	// Start server
	cfg.PrintServerStartMessage()
	if err := http.ListenAndServe(cfg.GetServerAddress(), nil); err != nil {
		log.Fatal(err)
	}
}
