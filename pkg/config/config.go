package config

import (
	"errors"
	"fmt"
	"os"
)

// Config holds all configuration for the application
type Config struct {
	SecretKey  string
	BucketName string
	Port       string
	TMDbAPIKey string

	// StorageBackend selects which StorageRepository implementation to use: "gcs" (default) or "r2".
	StorageBackend    string
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
}

// ErrSecretKeyNotSet is returned when the SECRET_KEY environment variable is not set
var ErrSecretKeyNotSet = errors.New("SECRET_KEY environment variable not set")

// ErrBucketNameNotSet is returned when the BUCKET_NAME environment variable is not set
var ErrBucketNameNotSet = errors.New("BUCKET_NAME environment variable not set")

// ErrR2AccountIDNotSet is returned when STORAGE_BACKEND=r2 and R2_ACCOUNT_ID is not set
var ErrR2AccountIDNotSet = errors.New("R2_ACCOUNT_ID environment variable not set")

// ErrR2AccessKeyIDNotSet is returned when STORAGE_BACKEND=r2 and R2_ACCESS_KEY_ID is not set
var ErrR2AccessKeyIDNotSet = errors.New("R2_ACCESS_KEY_ID environment variable not set")

// ErrR2SecretAccessKeyNotSet is returned when STORAGE_BACKEND=r2 and R2_SECRET_ACCESS_KEY is not set
var ErrR2SecretAccessKeyNotSet = errors.New("R2_SECRET_ACCESS_KEY environment variable not set")

// Load loads configuration from environment variables
func Load() (*Config, error) {
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		return nil, ErrSecretKeyNotSet
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		return nil, ErrBucketNameNotSet
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	tmdbAPIKey := os.Getenv("TMDB_API_KEY")

	storageBackend := os.Getenv("STORAGE_BACKEND")
	if storageBackend == "" {
		storageBackend = "gcs"
	}

	cfg := &Config{
		SecretKey:      secretKey,
		BucketName:     bucketName,
		Port:           port,
		TMDbAPIKey:     tmdbAPIKey,
		StorageBackend: storageBackend,
	}

	if storageBackend == "r2" {
		cfg.R2AccountID = os.Getenv("R2_ACCOUNT_ID")
		if cfg.R2AccountID == "" {
			return nil, ErrR2AccountIDNotSet
		}

		cfg.R2AccessKeyID = os.Getenv("R2_ACCESS_KEY_ID")
		if cfg.R2AccessKeyID == "" {
			return nil, ErrR2AccessKeyIDNotSet
		}

		cfg.R2SecretAccessKey = os.Getenv("R2_SECRET_ACCESS_KEY")
		if cfg.R2SecretAccessKey == "" {
			return nil, ErrR2SecretAccessKeyNotSet
		}
	}

	return cfg, nil
}

// ServerAddress returns the server address with port
func (c *Config) ServerAddress() string {
	return fmt.Sprintf(":%s", c.Port)
}

// PrintServerStartMessage prints a message when the server starts
func (c *Config) PrintServerStartMessage() {
	fmt.Printf("Starting server at port %s\n", c.Port)
	fmt.Printf("Gallery URL: http://localhost:%s/<SECRET_KEY>/index\n", c.Port)
	fmt.Printf("Feed URL: http://localhost:%s/<SECRET_KEY>/feed\n", c.Port)
	fmt.Printf("Admin URL: http://localhost:%s/<SECRET_KEY>/admin\n", c.Port)
}
