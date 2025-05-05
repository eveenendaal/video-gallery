package config

import (
	"fmt"
	"log"
	"os"
)

// Config holds all configuration for the application
type Config struct {
	SecretKey  string
	BucketName string
	Port       string
}

// Load loads configuration from environment variables
func Load() *Config {
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		panic("SECRET_KEY not set")
	}
	log.Println("Starting with Key: " + secretKey)

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		panic("BUCKET_NAME not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		SecretKey:  secretKey,
		BucketName: bucketName,
		Port:       port,
	}
}

// GetSecretKey returns the secret key
func (c *Config) GetSecretKey() string {
	return c.SecretKey
}

// GetBucketName returns the bucket name
func (c *Config) GetBucketName() string {
	return c.BucketName
}

// GetPort returns the port
func (c *Config) GetPort() string {
	return c.Port
}

// GetServerAddress returns the server address with port
func (c *Config) GetServerAddress() string {
	return ":" + c.Port
}

// PrintServerStartMessage prints a message when the server starts
func (c *Config) PrintServerStartMessage() {
	fmt.Printf("Starting server at port %s\n", c.Port)
	fmt.Printf("Access the application at: http://localhost:%s\n", c.Port)
	fmt.Printf("Gallery URL: http://localhost:%s/gallery/\n", c.Port)
	fmt.Printf("Admin URL: http://localhost:%s/%s/index\n", c.Port, c.SecretKey)
}
