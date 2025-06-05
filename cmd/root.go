package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"video-gallery/pkg/config"
)

// Configuration flags
var (
	secretKey  string
	bucketName string
	portNumber string
)

// NewRootCmd creates and returns the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "video-gallery",
		Short: "Video Gallery is a tool for managing and displaying video galleries",
		Long: `Video Gallery is a command line application that can display and manage video galleries
stored in Google Cloud Storage. It can also serve these galleries via a web interface.`,
	}

	// Define persistent flags that will be available for all commands
	rootCmd.PersistentFlags().StringVarP(&secretKey, "secret-key", "s", "", "Set the SECRET_KEY (overrides environment variable)")
	rootCmd.PersistentFlags().StringVarP(&bucketName, "bucket", "b", "", "Set the BUCKET_NAME (overrides environment variable)")
	rootCmd.PersistentFlags().StringVarP(&portNumber, "port", "p", "", "Set the PORT (overrides environment variable)")

	// Add commands to root
	rootCmd.AddCommand(newListCategoriesCmd())
	rootCmd.AddCommand(newListGalleriesCmd())
	rootCmd.AddCommand(newShowGalleryCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newGenerateThumbnailsCmd())

	return rootCmd
}

// LoadConfig loads configuration with respect to command line flags
func LoadConfig() (*config.Config, error) {
	// Set environment variables from flags if provided
	if secretKey != "" {
		os.Setenv("SECRET_KEY", secretKey)
	}

	if bucketName != "" {
		os.Setenv("BUCKET_NAME", bucketName)
	}

	if portNumber != "" {
		os.Setenv("PORT", portNumber)
	}

	// Load configuration from environment variables (potentially set above)
	return config.Load()
}
