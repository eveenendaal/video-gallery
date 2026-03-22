package application

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"video-gallery/internal/domain/gallery"
)

const (
	tmdbImageBaseW500 = "https://image.tmdb.org/t/p/w500"
	tmdbImageBaseW185 = "https://image.tmdb.org/t/p/w185"
)

// MoviePosterResult represents a movie poster search result returned to callers
type MoviePosterResult struct {
	Title        string `json:"title"`
	Year         string `json:"year"`
	PosterURL    string `json:"posterUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

// PosterService handles movie poster search and upload operations
type PosterService struct {
	repo           gallery.StorageRepository
	client         gallery.MoviePosterClient
	galleryService *GalleryService
}

// NewPosterService creates a new PosterService
func NewPosterService(
	repo gallery.StorageRepository,
	client gallery.MoviePosterClient,
	gallerySvc *GalleryService,
) *PosterService {
	return &PosterService{
		repo:           repo,
		client:         client,
		galleryService: gallerySvc,
	}
}

// FetchMoviePoster downloads a movie poster and stores it as the video's thumbnail.
// If posterURL is non-empty it is used directly; otherwise a TMDb search is performed.
func (s *PosterService) FetchMoviePoster(videoPath, movieTitle, posterURL string, progressCb ProgressCallback) error {
	send := func(step string, progress int) {
		if progressCb != nil {
			progressCb(step, progress)
		}
	}

	var actualPosterURL string

	if posterURL != "" {
		send("Using selected poster", 30)
		actualPosterURL = posterURL
	} else {
		send("Searching for movie", 15)

		cleanTitle := cleanMovieTitle(movieTitle)
		results, err := s.client.SearchMovies(context.Background(), cleanTitle)
		if err != nil {
			return fmt.Errorf("failed to search movie: %v", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("no movie found for title: %s", movieTitle)
		}

		movie := findBestMatch(results, cleanTitle)
		if movie.PosterPath == nil || *movie.PosterPath == "" {
			return fmt.Errorf("no poster available for: %s", movieTitle)
		}

		actualPosterURL = tmdbImageBaseW500 + *movie.PosterPath
	}

	send("Downloading poster", 40)

	outputDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	ext := filepath.Ext(videoPath)
	basePath := videoPath[:len(videoPath)-len(ext)]
	thumbnailPath := basePath + ".jpg"

	tmpThumbnailPath := filepath.Join(outputDir, safeFilename(thumbnailPath))

	if err := s.client.DownloadImage(context.Background(), actualPosterURL, tmpThumbnailPath); err != nil {
		return fmt.Errorf("failed to download poster: %v", err)
	}
	defer os.Remove(tmpThumbnailPath)

	send("Uploading to storage", 85)

	// Remove any existing thumbnail before uploading the new one
	_ = s.repo.DeleteObject(context.Background(), thumbnailPath)

	if err := s.repo.UploadObject(context.Background(), tmpThumbnailPath, thumbnailPath); err != nil {
		return fmt.Errorf("error uploading poster: %v", err)
	}

	send("Clearing cache", 95)
	s.galleryService.InvalidateCache()

	send("Complete", 100)
	log.Printf("Successfully fetched poster for: %s", movieTitle)
	return nil
}

// SearchMoviePoster searches for a movie and returns a list of available poster options
func (s *PosterService) SearchMoviePoster(movieTitle string) ([]MoviePosterResult, error) {
	cleanTitle := cleanMovieTitle(movieTitle)
	results, err := s.client.SearchMovies(context.Background(), cleanTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %v", err)
	}

	var posters []MoviePosterResult
	for _, movie := range results {
		if movie.PosterPath != nil && *movie.PosterPath != "" {
			posters = append(posters, MoviePosterResult{
				Title:        movie.Title,
				Year:         extractYear(movie.ReleaseDate),
				PosterURL:    tmdbImageBaseW500 + *movie.PosterPath,
				ThumbnailURL: tmdbImageBaseW185 + *movie.PosterPath,
			})
		}
	}
	return posters, nil
}

// cleanMovieTitle removes common metadata suffixes from a movie title so that
// searches return better results.  E.g. "Empire Strikes Back (Despecialized v2.0)"
// becomes "Empire Strikes Back".
func cleanMovieTitle(title string) string {
	if idx := strings.Index(title, "("); idx != -1 {
		title = title[:idx]
	}
	if idx := strings.Index(title, "["); idx != -1 {
		title = title[:idx]
	}
	return strings.TrimSpace(title)
}

// findBestMatch picks the most relevant movie from a search result list.
// Preference order: exact title match → partial match → first result.
func findBestMatch(results []gallery.MovieResult, searchTitle string) gallery.MovieResult {
	searchLower := strings.ToLower(searchTitle)

	for _, movie := range results {
		if strings.ToLower(movie.Title) == searchLower {
			return movie
		}
	}
	for _, movie := range results {
		if strings.Contains(strings.ToLower(movie.Title), searchLower) {
			return movie
		}
	}
	return results[0]
}

func extractYear(releaseDate string) string {
	if len(releaseDate) >= 4 {
		return releaseDate[:4]
	}
	return ""
}
