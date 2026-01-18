package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
)

const (
	tmdbSearchURL = "https://api.themoviedb.org/3/search/movie"
	tmdbImageBase = "https://image.tmdb.org/t/p/w500"
)

// TMDbSearchResult represents a movie search result from TMDb
type TMDbSearchResult struct {
	Results []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		PosterPath  *string `json:"poster_path"`
		ReleaseDate string  `json:"release_date"`
	} `json:"results"`
}

// FetchMoviePoster searches for a movie poster and uploads it to storage
func (s *Service) FetchMoviePoster(videoPath string, movieTitle string, progressCb ProgressCallback) error {
	sendProgress := func(step string, progress int) {
		if progressCb != nil {
			progressCb(step, progress)
		}
	}

	sendProgress("Getting API key", 5)
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("TMDB_API_KEY environment variable not set")
	}

	sendProgress("Searching for movie", 15)

	// Search for movie
	searchURL := fmt.Sprintf("%s?api_key=%s&query=%s", tmdbSearchURL, apiKey, url.QueryEscape(movieTitle))
	resp, err := http.Get(searchURL)
	if err != nil {
		return fmt.Errorf("failed to search movie: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TMDb API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result TMDbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode search result: %v", err)
	}

	if len(result.Results) == 0 {
		return fmt.Errorf("no movie found for title: %s", movieTitle)
	}

	movie := result.Results[0]
	if movie.PosterPath == nil || *movie.PosterPath == "" {
		return fmt.Errorf("no poster available for: %s", movieTitle)
	}

	sendProgress("Downloading poster", 40)

	// Download poster image
	posterURL := tmdbImageBase + *movie.PosterPath
	posterResp, err := http.Get(posterURL)
	if err != nil {
		return fmt.Errorf("failed to download poster: %v", err)
	}
	defer posterResp.Body.Close()

	if posterResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download poster (status %d)", posterResp.StatusCode)
	}

	sendProgress("Saving poster", 70)

	// Save to temp file
	outputDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	ext := filepath.Ext(videoPath)
	basePath := videoPath[:len(videoPath)-len(ext)]
	thumbnailPath := basePath + ".jpg"

	thumbnailBaseName := getSafeFilename(thumbnailPath)
	tmpThumbnailPath := filepath.Join(outputDir, thumbnailBaseName)

	tmpFile, err := os.Create(tmpThumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()
	defer os.Remove(tmpThumbnailPath)

	if _, err := io.Copy(tmpFile, posterResp.Body); err != nil {
		return fmt.Errorf("failed to save poster: %v", err)
	}
	tmpFile.Close()

	sendProgress("Uploading to storage", 85)

	// Upload to cloud storage
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.BucketName)

	// Clear old thumbnail
	bucket.Object(thumbnailPath).Delete(ctx)

	if err := uploadFile(ctx, bucket, tmpThumbnailPath, thumbnailPath); err != nil {
		return fmt.Errorf("error uploading poster: %v", err)
	}

	sendProgress("Clearing cache", 95)
	s.videoCache.Flush()

	sendProgress("Complete", 100)
	log.Printf("Successfully fetched poster for: %s", movieTitle)
	return nil
}

// SearchMoviePoster searches for a movie and returns available posters
func (s *Service) SearchMoviePoster(movieTitle string) ([]MoviePosterResult, error) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY environment variable not set")
	}

	searchURL := fmt.Sprintf("%s?api_key=%s&query=%s", tmdbSearchURL, apiKey, url.QueryEscape(movieTitle))
	resp, err := http.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TMDb API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result TMDbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %v", err)
	}

	var posters []MoviePosterResult
	for _, movie := range result.Results {
		if movie.PosterPath != nil && *movie.PosterPath != "" {
			posters = append(posters, MoviePosterResult{
				Title:        movie.Title,
				Year:         extractYear(movie.ReleaseDate),
				PosterURL:    tmdbImageBase + *movie.PosterPath,
				ThumbnailURL: strings.Replace(tmdbImageBase, "w500", "w185", 1) + *movie.PosterPath,
			})
		}
	}

	return posters, nil
}

// MoviePosterResult represents a movie poster search result
type MoviePosterResult struct {
	Title        string `json:"title"`
	Year         string `json:"year"`
	PosterURL    string `json:"posterUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

func extractYear(releaseDate string) string {
	if len(releaseDate) >= 4 {
		return releaseDate[:4]
	}
	return ""
}
