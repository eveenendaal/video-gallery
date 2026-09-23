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
	cfg := &Config{
		SecretKey:      os.Getenv("SECRET_KEY"),
		BucketName:     os.Getenv("BUCKET_NAME"),
		Port:           getEnvOrDefault("PORT", "8080"),
		TMDbAPIKey:     os.Getenv("TMDB_API_KEY"),
		StorageBackend: getEnvOrDefault("STORAGE_BACKEND", "gcs"),
	}
	if cfg.StorageBackend == "r2" {
		cfg.R2AccountID = os.Getenv("R2_ACCOUNT_ID")
		cfg.R2AccessKeyID = os.Getenv("R2_ACCESS_KEY_ID")
		cfg.R2SecretAccessKey = os.Getenv("R2_SECRET_ACCESS_KEY")
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate returns the error for the first required setting that is missing
func (c *Config) validate() error {
	type setting struct {
		value string
		err   error
	}
	required := []setting{
		{c.SecretKey, ErrSecretKeyNotSet},
		{c.BucketName, ErrBucketNameNotSet},
	}
	if c.StorageBackend == "r2" {
		required = append(required,
			setting{c.R2AccountID, ErrR2AccountIDNotSet},
			setting{c.R2AccessKeyID, ErrR2AccessKeyIDNotSet},
			setting{c.R2SecretAccessKey, ErrR2SecretAccessKeyNotSet},
		)
	}
	for _, s := range required {
		if s.value == "" {
			return s.err
		}
	}
	return nil
}

func getEnvOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
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
