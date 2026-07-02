package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"video-gallery/internal/domain/gallery"
)

const (
	tmdbSearchURL = "https://api.themoviedb.org/3/search/movie"
	tmdbMovieURL  = "https://api.themoviedb.org/3/movie"

	// tmdbImageHost is the only host images may be downloaded from.
	tmdbImageHost = "image.tmdb.org"

	// maxImageBytes caps the size of a downloaded poster to avoid unbounded
	// memory/disk use from a malicious or oversized response.
	maxImageBytes = 25 * 1024 * 1024 // 25 MiB

	// httpTimeout bounds outbound HTTP requests so a slow or hostile endpoint
	// cannot hang a request indefinitely.
	httpTimeout = 30 * time.Second
)

// httpClient is a shared client with a bounded timeout for all outbound requests.
var httpClient = &http.Client{Timeout: httpTimeout}

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
	resp, err := httpClient.Get(searchURL) // #nosec G107 -- URL is constructed from a trusted constant + encoded user input
	if err != nil {
		return nil, fmt.Errorf("failed to search TMDb: %v", scrubURLError(err))
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

// GetMovie fetches a single movie by its TMDb ID. Looking movies up by numeric
// ID (rather than accepting a caller-supplied URL) ensures poster downloads
// only ever use URLs constructed from TMDb's own API responses.
func (c *Client) GetMovie(_ context.Context, id int) (gallery.MovieResult, error) {
	apiKey := c.resolveAPIKey()
	if apiKey == "" {
		return gallery.MovieResult{}, fmt.Errorf("TMDB_API_KEY is not set")
	}

	movieURL := fmt.Sprintf("%s/%d?api_key=%s", tmdbMovieURL, id, apiKey)
	resp, err := httpClient.Get(movieURL) // #nosec G107 -- URL is a trusted constant plus an integer ID
	if err != nil {
		return gallery.MovieResult{}, fmt.Errorf("failed to fetch movie from TMDb: %v", scrubURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return gallery.MovieResult{}, fmt.Errorf("TMDb API error (status %d)", resp.StatusCode)
	}

	var raw struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		PosterPath  *string `json:"poster_path"`
		ReleaseDate string  `json:"release_date"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return gallery.MovieResult{}, fmt.Errorf("failed to decode TMDb response: %v", err)
	}

	return gallery.MovieResult{
		ID:          raw.ID,
		Title:       raw.Title,
		PosterPath:  raw.PosterPath,
		ReleaseDate: raw.ReleaseDate,
	}, nil
}

// DownloadImage downloads an image from imageURL and saves it to destPath.
// imageURL must be an HTTPS URL on the TMDb image host; this guards against
// SSRF in case an unvalidated URL ever reaches this method.
func (c *Client) DownloadImage(_ context.Context, imageURL, destPath string) error {
	if err := validateImageURL(imageURL); err != nil {
		return err
	}

	resp, err := httpClient.Get(imageURL) // #nosec G107 -- validateImageURL restricts the host to the TMDb image CDN
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

	// Cap the number of bytes read so an oversized response cannot exhaust disk/memory.
	limited := io.LimitReader(resp.Body, maxImageBytes+1)
	written, err := io.Copy(f, limited)
	if err != nil {
		return fmt.Errorf("failed to write image data: %v", err)
	}
	if written > maxImageBytes {
		return fmt.Errorf("image exceeds maximum allowed size of %d bytes", maxImageBytes)
	}
	return nil
}

// validateImageURL ensures imageURL uses HTTPS and targets the TMDb image host.
func validateImageURL(imageURL string) error {
	u, err := url.Parse(imageURL)
	if err != nil {
		return fmt.Errorf("invalid image URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("image URL must use https")
	}
	if !strings.EqualFold(u.Hostname(), tmdbImageHost) {
		return fmt.Errorf("image URL host is not allowed")
	}
	return nil
}

func (c *Client) resolveAPIKey() string {
	if c.apiKey != "" {
		return c.apiKey
	}
	return os.Getenv("TMDB_API_KEY")
}

// scrubURLError strips the request URL from an *url.Error so that the api_key
// query parameter is never included in a returned or logged error message.
func scrubURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
