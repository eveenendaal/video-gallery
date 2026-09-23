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
	"strconv"
	"strings"
	"time"

	"video-gallery/internal/domain/gallery"
)

const (
	tmdbAPIBaseURL = "https://api.themoviedb.org/3"

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

// tmdbMovie is the raw movie shape shared by the TMDb search and movie APIs
type tmdbMovie struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	PosterPath  *string `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
}

func (m tmdbMovie) toResult() gallery.MovieResult {
	return gallery.MovieResult{
		ID:          m.ID,
		Title:       m.Title,
		PosterPath:  m.PosterPath,
		ReleaseDate: m.ReleaseDate,
	}
}

// Client is a TMDb-backed implementation of gallery.MoviePosterClient
type Client struct {
	apiKey  string
	baseURL string
}

// NewClient creates a new TMDb Client. An empty apiKey is allowed so the
// server can start without one; lookups then fail with a clear error.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, baseURL: tmdbAPIBaseURL}
}

// SearchMovies queries the TMDb API for movies matching title
func (c *Client) SearchMovies(ctx context.Context, title string) ([]gallery.MovieResult, error) {
	var raw struct {
		Results []tmdbMovie `json:"results"`
	}
	if err := c.getJSON(ctx, "/search/movie", url.Values{"query": {title}}, &raw); err != nil {
		return nil, err
	}

	results := make([]gallery.MovieResult, 0, len(raw.Results))
	for _, m := range raw.Results {
		results = append(results, m.toResult())
	}
	return results, nil
}

// GetMovie fetches a single movie by its TMDb ID. Looking movies up by numeric
// ID (rather than accepting a caller-supplied URL) ensures poster downloads
// only ever use URLs constructed from TMDb's own API responses.
func (c *Client) GetMovie(ctx context.Context, id int) (gallery.MovieResult, error) {
	var raw tmdbMovie
	if err := c.getJSON(ctx, "/movie/"+strconv.Itoa(id), url.Values{}, &raw); err != nil {
		return gallery.MovieResult{}, err
	}
	return raw.toResult(), nil
}

// getJSON performs an authenticated GET against the TMDb API and decodes the
// JSON response into v.
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, v any) error {
	if c.apiKey == "" {
		return fmt.Errorf("TMDB_API_KEY is not set")
	}
	query.Set("api_key", c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("failed to build TMDb request: %v", scrubURLError(err))
	}
	resp, err := httpClient.Do(req) // #nosec G107 -- URL is a trusted base plus an encoded path/query
	if err != nil {
		return fmt.Errorf("TMDb request failed: %v", scrubURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDb API error (status %d)", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode TMDb response: %v", err)
	}
	return nil
}

// DownloadImage downloads an image from imageURL and saves it to destPath.
// imageURL must be an HTTPS URL on the TMDb image host; this guards against
// SSRF in case an unvalidated URL ever reaches this method.
func (c *Client) DownloadImage(ctx context.Context, imageURL, destPath string) error {
	if err := validateImageURL(imageURL); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build image request: %v", err)
	}
	resp, err := httpClient.Do(req) // #nosec G107 -- validateImageURL restricts the host to the TMDb image CDN
	if err != nil {
		return fmt.Errorf("failed to download image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image download failed (status %d)", resp.StatusCode)
	}
	return writeLimited(resp.Body, destPath, maxImageBytes)
}

// writeLimited copies r to a new file at destPath, failing if r holds more
// than limit bytes so an oversized response cannot exhaust disk/memory.
func writeLimited(r io.Reader, destPath string, limit int64) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer f.Close()

	written, err := io.Copy(f, io.LimitReader(r, limit+1))
	if err != nil {
		return fmt.Errorf("failed to write image data: %v", err)
	}
	if written > limit {
		return fmt.Errorf("image exceeds maximum allowed size of %d bytes", limit)
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

// scrubURLError strips the request URL from an *url.Error so that the api_key
// query parameter is never included in a returned or logged error message.
func scrubURLError(err error) error {
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return urlErr.Err
	}
	return err
}
