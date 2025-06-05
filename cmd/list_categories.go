package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"video-gallery/pkg/services"
)

// newListCategoriesCmd creates a new command for listing categories
func newListCategoriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-categories",
		Short: "List all video categories",
		Long:  `List all video categories with the number of galleries in each.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}
			services.InitService(cfg)
			listCategories()
		},
	}
}

// listCategories displays all categories and their galleries
func listCategories() {
	categories := services.GetCategories()

	fmt.Println("Video Categories:")
	fmt.Println("================")

	for _, category := range categories {
		fmt.Printf("%s\n", category.Name)
		fmt.Printf("  Galleries: %d\n", len(category.Galleries))
		fmt.Println()
	}

	fmt.Printf("Total: %d categories\n", len(categories))
}
