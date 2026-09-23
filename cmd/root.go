package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"video-gallery/pkg/config"
)

// Configuration flags
var (
	secretKey      string
	bucketName     string
	portNumber     string
	storageBackend string
)

// Version of the application (set at build time)
var Version = "dev"

// NewRootCmd creates and returns the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "video-gallery",
		Short:   "Video Gallery - web server for video galleries",
		Long:    `Video Gallery is a web server that displays video galleries stored in a Google Cloud Storage or Cloudflare R2 bucket.`,
		Version: Version,
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	// Define persistent flags that will be available for all commands
	rootCmd.PersistentFlags().StringVarP(&secretKey, "secret-key", "s", "", "Set the SECRET_KEY (overrides environment variable)")
	rootCmd.PersistentFlags().StringVarP(&bucketName, "bucket", "b", "", "Set the BUCKET_NAME (overrides environment variable)")
	rootCmd.PersistentFlags().StringVarP(&portNumber, "port", "p", "", "Set the PORT (overrides environment variable)")
	rootCmd.PersistentFlags().StringVar(&storageBackend, "storage-backend", "", "Set the STORAGE_BACKEND: gcs or r2 (overrides environment variable)")

	// Add commands to root
	rootCmd.AddCommand(newServeCmd())

	return rootCmd
}

// LoadConfig loads configuration from environment variables, with any
// command line flags that were set taking precedence
func LoadConfig() (*config.Config, error) {
	overrides := map[string]string{
		"SECRET_KEY":      secretKey,
		"BUCKET_NAME":     bucketName,
		"PORT":            portNumber,
		"STORAGE_BACKEND": storageBackend,
	}
	for name, value := range overrides {
		if value != "" {
			os.Setenv(name, value)
		}
	}
	return config.Load()
}
