package application

import (
	"context"
	"fmt"
	"log"
	"strings"

	"video-gallery/internal/domain/gallery"
)

const (
	tmdbImageBaseW500 = "https://image.tmdb.org/t/p/w500"
	tmdbImageBaseW185 = "https://image.tmdb.org/t/p/w185"
)

// MoviePosterResult represents a movie poster search result returned to callers
type MoviePosterResult struct {
	ID           int    `json:"id"`
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
// If movieID is positive the poster is looked up by TMDb ID; otherwise a TMDb
// title search is performed. The poster URL is always constructed server-side
// from TMDb API responses, so callers can never make the server fetch an
// arbitrary URL (SSRF).
func (s *PosterService) FetchMoviePoster(videoPath, movieTitle string, movieID int, progressCb ProgressCallback) error {
	ctx := context.Background()
	var movie gallery.MovieResult

	if movieID > 0 {
		progressCb.report("Fetching selected movie", 15)
		var err error
		if movie, err = s.client.GetMovie(ctx, movieID); err != nil {
			return fmt.Errorf("failed to fetch movie: %v", err)
		}
	} else {
		progressCb.report("Searching for movie", 15)
		cleanTitle := cleanMovieTitle(movieTitle)
		results, err := s.client.SearchMovies(ctx, cleanTitle)
		if err != nil {
			return fmt.Errorf("failed to search movie: %v", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("no movie found for title: %s", movieTitle)
		}
		movie = findBestMatch(results, cleanTitle)
	}

	if movie.PosterPath == nil || *movie.PosterPath == "" {
		return fmt.Errorf("no poster available for: %s", movieTitle)
	}

	progressCb.report("Downloading poster", 40)
	tmpPoster, err := newTempFile(".jpg")
	if err != nil {
		return err
	}
	defer removeTempFile(tmpPoster)
	if err := s.client.DownloadImage(ctx, tmdbImageBaseW500+*movie.PosterPath, tmpPoster); err != nil {
		return fmt.Errorf("failed to download poster: %v", err)
	}

	progressCb.report("Uploading to storage", 85)
	if err := s.repo.UploadObject(ctx, tmpPoster, gallery.ThumbnailPathFor(videoPath)); err != nil {
		return fmt.Errorf("error uploading poster: %v", err)
	}

	progressCb.report("Clearing cache", 95)
	s.galleryService.InvalidateCache()

	progressCb.report("Complete", 100)
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

	posters := []MoviePosterResult{}
	for _, movie := range results {
		if movie.PosterPath == nil || *movie.PosterPath == "" {
			continue
		}
		posters = append(posters, MoviePosterResult{
			ID:           movie.ID,
			Title:        movie.Title,
			Year:         extractYear(movie.ReleaseDate),
			PosterURL:    tmdbImageBaseW500 + *movie.PosterPath,
			ThumbnailURL: tmdbImageBaseW185 + *movie.PosterPath,
		})
	}
	return posters, nil
}

// cleanMovieTitle removes common metadata suffixes from a movie title so that
// searches return better results.  E.g. "Empire Strikes Back (Despecialized v2.0)"
// becomes "Empire Strikes Back".
func cleanMovieTitle(title string) string {
	if idx := strings.IndexAny(title, "(["); idx != -1 {
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
