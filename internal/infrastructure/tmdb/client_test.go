package tmdb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateImageURL(t *testing.T) {
	allowed := []string{
		"https://image.tmdb.org/t/p/w500/abc.jpg",
		"https://IMAGE.TMDB.ORG/t/p/w185/def.jpg",
	}
	for _, u := range allowed {
		if err := validateImageURL(u); err != nil {
			t.Errorf("expected %q to be allowed, got error: %v", u, err)
		}
	}

	blocked := []string{
		"http://image.tmdb.org/t/p/w500/abc.jpg",      // not https
		"https://169.254.169.254/computeMetadata/v1/", // cloud metadata
		"https://evil.com/x.jpg",                      // arbitrary host
		"https://image.tmdb.org.evil.com/x.jpg",       // suffix trick
		"https://image.tmdb.org@evil.com/x.jpg",       // userinfo trick
		"file:///etc/passwd",                          // non-http scheme
		"",                                            // empty
	}
	for _, u := range blocked {
		if err := validateImageURL(u); err == nil {
			t.Errorf("expected %q to be blocked, but it was allowed", u)
		}
	}
}

// newTestClient returns a Client pointed at a test server running handler
func newTestClient(t *testing.T, apiKey string, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(apiKey)
	c.baseURL = srv.URL
	return c
}

func TestSearchMovies(t *testing.T) {
	c := newTestClient(t, "key123", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/movie" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "Alien & Co" {
			t.Errorf("query = %q, want the unescaped title", got)
		}
		if got := r.URL.Query().Get("api_key"); got != "key123" {
			t.Errorf("api_key = %q", got)
		}
		io.WriteString(w, `{"results":[
			{"id":1,"title":"Alien","poster_path":"/a.jpg","release_date":"1979-05-25"},
			{"id":2,"title":"No Poster","poster_path":null}
		]}`)
	})

	results, err := c.SearchMovies(context.Background(), "Alien & Co")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %+v", results)
	}
	if r := results[0]; r.ID != 1 || r.Title != "Alien" || r.PosterPath == nil || *r.PosterPath != "/a.jpg" || r.ReleaseDate != "1979-05-25" {
		t.Errorf("unexpected first result: %+v", r)
	}
	if results[1].PosterPath != nil {
		t.Error("null poster_path should decode to nil")
	}
}

func TestGetMovie(t *testing.T) {
	c := newTestClient(t, "key123", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/348" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		io.WriteString(w, `{"id":348,"title":"Alien","poster_path":"/a.jpg"}`)
	})

	movie, err := c.GetMovie(context.Background(), 348)
	if err != nil || movie.ID != 348 || movie.Title != "Alien" {
		t.Fatalf("got %+v, %v", movie, err)
	}
}

func TestAPIErrors(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		c := newTestClient(t, "", func(http.ResponseWriter, *http.Request) {
			t.Error("no request should be made without an API key")
		})
		if _, err := c.SearchMovies(context.Background(), "x"); err == nil {
			t.Error("expected error")
		}
		if _, err := c.GetMovie(context.Background(), 1); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("bad status", func(t *testing.T) {
		c := newTestClient(t, "key", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		})
		_, err := c.SearchMovies(context.Background(), "x")
		if err == nil || !strings.Contains(err.Error(), "401") {
			t.Errorf("expected status error, got %v", err)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		c := newTestClient(t, "key", func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "not json")
		})
		if _, err := c.GetMovie(context.Background(), 1); err == nil {
			t.Error("expected decode error")
		}
	})

	t.Run("transport error does not leak key", func(t *testing.T) {
		c := NewClient("super-secret-key")
		c.baseURL = "http://127.0.0.1:1" // nothing listens here
		_, err := c.SearchMovies(context.Background(), "x")
		if err == nil || strings.Contains(err.Error(), "super-secret-key") {
			t.Errorf("expected an error without the API key, got %v", err)
		}
	})
}

func TestDownloadImageRejectsDisallowedURL(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "poster.jpg")
	if err := NewClient("key").DownloadImage(context.Background(), "https://evil.com/x.jpg", dest); err == nil {
		t.Fatal("expected disallowed host to be rejected")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("no file should be created for a rejected URL")
	}
}

func TestWriteLimited(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "img")

	if err := writeLimited(strings.NewReader("12345"), dest, 5); err != nil {
		t.Fatalf("data at the limit should be accepted: %v", err)
	}
	if data, _ := os.ReadFile(dest); string(data) != "12345" {
		t.Errorf("unexpected file contents %q", data)
	}

	if err := writeLimited(strings.NewReader("123456"), dest, 5); err == nil {
		t.Error("data over the limit should be rejected")
	}
}
