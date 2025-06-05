package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"video-gallery/pkg/services"
)

// newShowGalleryCmd creates a new command for showing gallery details
func newShowGalleryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show-gallery [stub]",
		Short: "Show videos in a specific gallery",
		Long:  `Show detailed information about videos in a specific gallery identified by its stub.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}
			services.InitService(cfg)
			showGallery(args[0])
		},
	}
}

// showGallery displays details about a specific gallery
func showGallery(stub string) {
	gallery, err := services.GetGallery(stub)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Gallery: %s\n", gallery.Name)
	fmt.Printf("Category: %s\n", gallery.Category)
	fmt.Printf("Videos: %d\n", len(gallery.Videos))
	fmt.Println("================")

	for i, video := range gallery.Videos {
		fmt.Printf("%d. %s\n", i+1, video.Name)
		fmt.Printf("   URL: %s\n", video.Url)
		if video.Thumbnail != nil {
			fmt.Printf("   Thumbnail: %s\n", *video.Thumbnail)
		}
		fmt.Println()
	}
}
