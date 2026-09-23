package application

import (
	"errors"
	"testing"

	"video-gallery/internal/domain/gallery"
)

func TestCleanMovieTitle(t *testing.T) {
	tests := map[string]string{
		"Empire Strikes Back (Despecialized v2.0)": "Empire Strikes Back",
		"Alien [Director's Cut]":                   "Alien",
		"Heat [1995] (Remastered)":                 "Heat",
		"  Plain Title  ":                          "Plain Title",
		"":                                         "",
	}
	for in, want := range tests {
		if got := cleanMovieTitle(in); got != want {
			t.Errorf("cleanMovieTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindBestMatch(t *testing.T) {
	results := []gallery.MovieResult{
		{ID: 1, Title: "Alien Resurrection"},
		{ID: 2, Title: "Aliens"},
		{ID: 3, Title: "Alien"},
	}
	if got := findBestMatch(results, "alien"); got.ID != 3 {
		t.Errorf("exact match should win, got %d", got.ID)
	}
	if got := findBestMatch(results, "resurrection"); got.ID != 1 {
		t.Errorf("partial match should win over first result, got %d", got.ID)
	}
	if got := findBestMatch(results, "Predator"); got.ID != 1 {
		t.Errorf("should fall back to first result, got %d", got.ID)
	}
}

func TestExtractYear(t *testing.T) {
	for in, want := range map[string]string{"1979-05-25": "1979", "1979": "1979", "79": "", "": ""} {
		if got := extractYear(in); got != want {
			t.Errorf("extractYear(%q) = %q, want %q", in, got, want)
		}
	}
}

func newPosterTestService(repo *fakeRepo, client *fakePosterClient) *PosterService {
	return NewPosterService(repo, client, NewGalleryService(repo, "secret"))
}

func TestFetchMoviePosterBySearch(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/Alien (1979).mp4")
	client := &fakePosterClient{search: []gallery.MovieResult{
		{ID: 1, Title: "Aliens", PosterPath: strPtr("/aliens.jpg")},
		{ID: 2, Title: "Alien", PosterPath: strPtr("/alien.jpg")},
	}}
	var progress progressLog

	err := newPosterTestService(repo, client).FetchMoviePoster("Movies/Classics/Alien (1979).mp4", "Alien (1979)", 0, progress.cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.searchedFor != "Alien" {
		t.Errorf("search should use cleaned title, got %q", client.searchedFor)
	}
	if client.downloaded != tmdbImageBaseW500+"/alien.jpg" {
		t.Errorf("should download best match's poster, got %q", client.downloaded)
	}
	if !repo.has("Movies/Classics/Alien (1979).jpg") {
		t.Error("poster should be uploaded as the video's thumbnail")
	}
	if progress.steps[len(progress.steps)-1] != 100 {
		t.Errorf("expected completion progress, got %v", progress.steps)
	}
}

func TestFetchMoviePosterByID(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/Film.mp4")
	client := &fakePosterClient{movies: map[int]gallery.MovieResult{
		42: {ID: 42, Title: "Chosen", PosterPath: strPtr("/chosen.jpg")},
	}}

	if err := newPosterTestService(repo, client).FetchMoviePoster("Movies/Classics/Film.mp4", "Film", 42, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.searchedFor != "" {
		t.Error("lookup by ID must not search")
	}
	if client.downloaded != tmdbImageBaseW500+"/chosen.jpg" {
		t.Errorf("unexpected poster URL %q", client.downloaded)
	}
}

func TestFetchMoviePosterErrors(t *testing.T) {
	tests := map[string]struct {
		client  *fakePosterClient
		movieID int
	}{
		"search error": {client: &fakePosterClient{searchErr: errors.New("boom")}},
		"no results":   {client: &fakePosterClient{}},
		"no poster":    {client: &fakePosterClient{search: []gallery.MovieResult{{ID: 1, Title: "Film"}}}},
		"empty poster": {client: &fakePosterClient{search: []gallery.MovieResult{{ID: 1, Title: "Film", PosterPath: strPtr("")}}}},
		"unknown id":   {client: &fakePosterClient{}, movieID: 7},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo("Movies/Classics/Film.mp4")
			err := newPosterTestService(repo, tt.client).FetchMoviePoster("Movies/Classics/Film.mp4", "Film", tt.movieID, nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			if repo.has("Movies/Classics/Film.jpg") {
				t.Error("nothing should be uploaded on failure")
			}
		})
	}
}

func TestSearchMoviePoster(t *testing.T) {
	client := &fakePosterClient{search: []gallery.MovieResult{
		{ID: 1, Title: "Alien", PosterPath: strPtr("/alien.jpg"), ReleaseDate: "1979-05-25"},
		{ID: 2, Title: "No Poster"},
	}}
	svc := newPosterTestService(newFakeRepo(), client)

	results, err := svc.SearchMoviePoster("Alien [4K]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.searchedFor != "Alien" {
		t.Errorf("search should use cleaned title, got %q", client.searchedFor)
	}
	want := MoviePosterResult{
		ID:           1,
		Title:        "Alien",
		Year:         "1979",
		PosterURL:    tmdbImageBaseW500 + "/alien.jpg",
		ThumbnailURL: tmdbImageBaseW185 + "/alien.jpg",
	}
	if len(results) != 1 || results[0] != want {
		t.Errorf("got %+v, want [%+v]", results, want)
	}

	client.search = nil
	if results, _ := svc.SearchMoviePoster("Nothing"); results == nil {
		t.Error("empty results should encode as [] rather than null")
	}

	client.searchErr = errors.New("boom")
	if _, err := svc.SearchMoviePoster("Alien"); err == nil {
		t.Error("expected search error to propagate")
	}
}
