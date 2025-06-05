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
}

// ErrSecretKeyNotSet is returned when the SECRET_KEY environment variable is not set
var ErrSecretKeyNotSet = errors.New("SECRET_KEY environment variable not set")

// ErrBucketNameNotSet is returned when the BUCKET_NAME environment variable is not set
var ErrBucketNameNotSet = errors.New("BUCKET_NAME environment variable not set")

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

	return &Config{
		SecretKey:  secretKey,
		BucketName: bucketName,
		Port:       port,
	}, nil
}

// ServerAddress returns the server address with port
func (c *Config) ServerAddress() string {
	return fmt.Sprintf(":%s", c.Port)
}

// PrintServerStartMessage prints a message when the server starts
func (c *Config) PrintServerStartMessage() {
	fmt.Printf("Starting server at port %s\n", c.Port)
	fmt.Printf("Gallery URL: http://localhost:%s/%s/index\n", c.Port, c.SecretKey)
	fmt.Printf("Feed URL: http://localhost:%s/%s/feed\n", c.Port, c.SecretKey)
}
