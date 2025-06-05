package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"video-gallery/pkg/services"
)

// newListGalleriesCmd creates a new command for listing galleries
func newListGalleriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-galleries",
		Short: "List all galleries",
		Long:  `List all galleries organized by category with the number of videos in each.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}
			services.InitService(cfg)
			listGalleries()
		},
	}
}

// listGalleries displays all galleries and their videos
func listGalleries() {
	categories := services.GetCategories()
	totalGalleries := 0

	fmt.Println("Video Galleries:")
	fmt.Println("===============")

	for _, category := range categories {
		fmt.Printf("Category: %s\n", category.Name)

		for _, gallery := range category.Galleries {
			fmt.Printf("  - %s (videos: %d)\n", gallery.Name, len(gallery.Videos))
			fmt.Printf("    Stub: %s\n", gallery.Stub)
			totalGalleries++
		}

		fmt.Println()
	}

	fmt.Printf("Total: %d galleries across %d categories\n", totalGalleries, len(categories))
}
