package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"video-gallery/internal/domain/gallery"
)

const (
	tmdbSearchURL = "https://api.themoviedb.org/3/search/movie"
)

// tmdbSearchResult is the raw response shape from the TMDb search API
type tmdbSearchResult struct {
	Results []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		PosterPath  *string `json:"poster_path"`
		ReleaseDate string  `json:"release_date"`
	} `json:"results"`
}

// Client is a TMDb-backed implementation of gallery.MoviePosterClient
type Client struct {
	apiKey string
}

// NewClient creates a new TMDb Client.
// If apiKey is empty the key is read from the TMDB_API_KEY environment variable
// at call time.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

// SearchMovies queries the TMDb API for movies matching title
func (c *Client) SearchMovies(_ context.Context, title string) ([]gallery.MovieResult, error) {
	apiKey := c.resolveAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB_API_KEY is not set")
	}

	searchURL := fmt.Sprintf("%s?api_key=%s&query=%s", tmdbSearchURL, apiKey, url.QueryEscape(title))
	resp, err := http.Get(searchURL) // #nosec G107 -- URL is constructed from trusted constant + encoded user input
	if err != nil {
		return nil, fmt.Errorf("failed to search TMDb: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TMDb API error (status %d): %s", resp.StatusCode, string(body))
	}

	var raw tmdbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode TMDb response: %v", err)
	}

	results := make([]gallery.MovieResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		results = append(results, gallery.MovieResult{
			ID:          r.ID,
			Title:       r.Title,
			PosterPath:  r.PosterPath,
			ReleaseDate: r.ReleaseDate,
		})
	}
	return results, nil
}

// DownloadImage downloads an image from imageURL and saves it to destPath
func (c *Client) DownloadImage(_ context.Context, imageURL, destPath string) error {
	resp, err := http.Get(imageURL) // #nosec G107 -- URL is a TMDb image URL provided by the application
	if err != nil {
		return fmt.Errorf("failed to download image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image download failed (status %d)", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write image data: %v", err)
	}
	return nil
}

func (c *Client) resolveAPIKey() string {
	if c.apiKey != "" {
		return c.apiKey
	}
	return os.Getenv("TMDB_API_KEY")
}
