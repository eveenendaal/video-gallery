package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"video-gallery/pkg/services"
)

// newExportCmd creates a new command for exporting gallery data
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export [format]",
		Short: "Export gallery data",
		Long:  `Export all gallery data in the specified format. Currently supported formats: json.`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}
			services.InitService(cfg)

			format := "json"
			if len(args) > 0 {
				format = args[0]
			}
			exportData(format)
		},
	}
}

// exportData exports gallery data in the specified format
func exportData(format string) {
	if format != "json" {
		fmt.Printf("Unsupported export format: %s\n", format)
		fmt.Println("Supported formats: json")
		os.Exit(1)
	}

	categories := services.GetCategories()

	// Sort categories by name for consistent output
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling data: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
