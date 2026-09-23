package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"video-gallery/internal/application"
	"video-gallery/internal/domain/gallery"
)

// TestMain runs from the repo root, where the server expects to find its templates
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// memRepo is a minimal in-memory gallery.StorageRepository
type memRepo map[string]string

func (r memRepo) ListObjects(context.Context) ([]gallery.StorageObject, error) {
	var objs []gallery.StorageObject
	for k := range r {
		objs = append(objs, gallery.StorageObject{Name: k})
	}
	return objs, nil
}

func (r memRepo) GetSignedURL(_ context.Context, path string, _ time.Duration) (string, error) {
	return "https://signed.example/" + url.PathEscape(path), nil
}

func (r memRepo) DeleteObject(_ context.Context, path string) error {
	if _, ok := r[path]; !ok {
		return errors.New("not found")
	}
	delete(r, path)
	return nil
}

func (r memRepo) DownloadObject(_ context.Context, remotePath, localPath string) error {
	data, ok := r[remotePath]
	if !ok {
		return fmt.Errorf("object %q not found", remotePath)
	}
	return os.WriteFile(localPath, []byte(data), 0o644)
}

func (r memRepo) UploadObject(_ context.Context, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	r[remotePath] = string(data)
	return err
}

type okProcessor struct{}

func (okProcessor) ExtractFrame(_, thumbnailPath string, _ int) error {
	return os.WriteFile(thumbnailPath, []byte("frame"), 0o644)
}
func (okProcessor) ValidateImage(string) error { return nil }

type noPosters struct{}

func (noPosters) SearchMovies(context.Context, string) ([]gallery.MovieResult, error) {
	return nil, nil
}
func (noPosters) GetMovie(context.Context, int) (gallery.MovieResult, error) {
	return gallery.MovieResult{}, errors.New("not found")
}
func (noPosters) DownloadImage(context.Context, string, string) error { return nil }

func newTestHandlers(repo memRepo) (*GalleryHandlers, *AdminHandlers, *application.GalleryService) {
	gallerySvc := application.NewGalleryService(repo, "secret")
	thumbnailSvc := application.NewThumbnailService(repo, okProcessor{}, gallerySvc)
	posterSvc := application.NewPosterService(repo, noPosters{}, gallerySvc)
	return NewGalleryHandlers(gallerySvc), NewAdminHandlers(gallerySvc, thumbnailSvc, posterSvc, "secret"), gallerySvc
}

func sampleRepo() memRepo {
	return memRepo{
		"Movies/Classics/Film.mp4": "video",
		"Movies/Classics/Film.jpg": "thumb",
		"Home/Bob/Clip.mp4":        "video",
	}
}

func serve(h http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

func TestIsValidObjectPath(t *testing.T) {
	valid := []string{"Movies/Classics/Film.mp4", "Movies/Classics/Film.MOV"}
	invalid := []string{
		"",
		"Film.mp4",
		"Movies/Film.mp4",
		"/Movies/Classics/Film.mp4",
		"Movies/Classics/Film.jpg",
		"Movies/../Film.mp4",
		"Movies/Classics/Film.mp4?x=1",
		"a/b/c/Film.mp4",
	}
	for _, p := range valid {
		if !isValidObjectPath(p, gallery.IsVideo) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	for _, p := range invalid {
		if isValidObjectPath(p, gallery.IsVideo) {
			t.Errorf("expected %q to be invalid", p)
		}
	}
}

func TestFeedHandler(t *testing.T) {
	pages, _, _ := newTestHandlers(sampleRepo())
	rec := serve(pages.FeedHandler, http.MethodGet, "/secret/feed", "")

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var feed []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body)
	}
	if len(feed) != 2 || feed[0]["name"] != "Bob" || feed[1]["name"] != "Classics" {
		t.Errorf("unexpected feed: %v", feed)
	}
	// Internal fields must not leak into the public feed.
	if strings.Contains(rec.Body.String(), "VideoPath") || strings.Contains(rec.Body.String(), "stub") {
		t.Errorf("feed leaks internal fields: %s", rec.Body)
	}
}

func TestIndexAndGalleryPages(t *testing.T) {
	pages, _, svc := newTestHandlers(sampleRepo())

	rec := serve(pages.IndexHandler, http.MethodGet, "/secret/index", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Classics") {
		t.Fatalf("index: status %d body %s", rec.Code, rec.Body)
	}

	g, _ := svc.GetGallery(svc.GetGalleries()[1].Stub)
	rec = serve(pages.PageHandler, http.MethodGet, g.Stub+"?utm=x", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Film") {
		t.Errorf("gallery page: status %d body %s", rec.Code, rec.Body)
	}

	rec = serve(pages.PageHandler, http.MethodGet, "/gallery/unknown", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown gallery: status %d, want 404", rec.Code)
	}
}

func TestAdminHandlerRendersSecretKey(t *testing.T) {
	_, admin, _ := newTestHandlers(sampleRepo())
	rec := serve(admin.AdminHandler, http.MethodGet, "/secret/admin", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "const secretKey = 'secret'") {
		t.Errorf("admin page: status %d", rec.Code)
	}
}

// sseEvents parses the data payloads of an SSE response body
func sseEvents(t *testing.T, body string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, chunk := range strings.Split(strings.TrimSpace(body), "\n\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(chunk, "data: ")), &ev); err != nil {
			t.Fatalf("bad SSE chunk %q: %v", chunk, err)
		}
		events = append(events, ev)
	}
	return events
}

func TestGenerateThumbnailHandler(t *testing.T) {
	repo := sampleRepo()
	_, admin, _ := newTestHandlers(repo)

	rec := serve(admin.GenerateThumbnailHandler, http.MethodGet,
		"/secret/admin/api/generate-thumbnail?videoPath="+url.QueryEscape("Home/Bob/Clip.mp4")+"&timeMs=500", "")
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, body %s", ct, rec.Body)
	}
	events := sseEvents(t, rec.Body.String())
	if last := events[len(events)-1]; last["progress"] != float64(100) {
		t.Errorf("expected final progress 100, got %v", events)
	}
	if repo["Home/Bob/Clip.jpg"] != "frame" {
		t.Error("thumbnail should be uploaded")
	}
}

func TestGenerateThumbnailHandlerStreamsErrors(t *testing.T) {
	_, admin, _ := newTestHandlers(sampleRepo())
	rec := serve(admin.GenerateThumbnailHandler, http.MethodGet,
		"/secret/admin/api/generate-thumbnail?videoPath="+url.QueryEscape("Home/Bob/Missing.mp4"), "")

	events := sseEvents(t, rec.Body.String())
	last := events[len(events)-1]
	if last["progress"] != float64(-1) || last["error"] == nil {
		t.Errorf("expected a final error event, got %v", events)
	}
}

func TestAdminValidation(t *testing.T) {
	_, admin, _ := newTestHandlers(sampleRepo())
	video := url.QueryEscape("Home/Bob/Clip.mp4")

	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		target  string
		body    string
		want    int
	}{
		{"generate: wrong method", admin.GenerateThumbnailHandler, http.MethodPost, "/x?videoPath=" + video, "", http.StatusMethodNotAllowed},
		{"generate: bad path", admin.GenerateThumbnailHandler, http.MethodGet, "/x?videoPath=%2Fetc%2Fpasswd", "", http.StatusBadRequest},
		{"generate: image path", admin.GenerateThumbnailHandler, http.MethodGet, "/x?videoPath=" + url.QueryEscape("Movies/Classics/Film.jpg"), "", http.StatusBadRequest},
		{"generate: negative time", admin.GenerateThumbnailHandler, http.MethodGet, "/x?timeMs=-1&videoPath=" + video, "", http.StatusBadRequest},
		{"generate: non-numeric time", admin.GenerateThumbnailHandler, http.MethodGet, "/x?timeMs=abc&videoPath=" + video, "", http.StatusBadRequest},
		{"poster: wrong method", admin.FetchMoviePosterHandler, http.MethodPost, "/x", "", http.StatusMethodNotAllowed},
		{"poster: missing title", admin.FetchMoviePosterHandler, http.MethodGet, "/x?videoPath=" + video, "", http.StatusBadRequest},
		{"poster: bad id", admin.FetchMoviePosterHandler, http.MethodGet, "/x?movieTitle=A&movieId=-3&videoPath=" + video, "", http.StatusBadRequest},
		{"poster: bad path", admin.FetchMoviePosterHandler, http.MethodGet, "/x?movieTitle=A&videoPath=nope.mp4", "", http.StatusBadRequest},
		{"search: wrong method", admin.SearchMoviePosterHandler, http.MethodPost, "/x", "", http.StatusMethodNotAllowed},
		{"search: missing title", admin.SearchMoviePosterHandler, http.MethodGet, "/x", "", http.StatusBadRequest},
		{"clear: wrong method", admin.ClearThumbnailHandler, http.MethodGet, "/x", "", http.StatusMethodNotAllowed},
		{"clear: bad json", admin.ClearThumbnailHandler, http.MethodPost, "/x", "{", http.StatusBadRequest},
		{"clear: video path", admin.ClearThumbnailHandler, http.MethodPost, "/x", `{"thumbnailPath":"Home/Bob/Clip.mp4"}`, http.StatusBadRequest},
		{"clear: oversized body", admin.ClearThumbnailHandler, http.MethodPost, "/x", `{"thumbnailPath":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}`, http.StatusBadRequest},
		{"bulk generate: wrong method", admin.BulkGenerateThumbnailsHandler, http.MethodGet, "/x", "", http.StatusMethodNotAllowed},
		{"bulk generate: negative time", admin.BulkGenerateThumbnailsHandler, http.MethodPost, "/x", `{"timeMs":-1}`, http.StatusBadRequest},
		{"bulk clear: wrong method", admin.BulkClearThumbnailsHandler, http.MethodGet, "/x", "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := serve(tt.handler, tt.method, tt.target, tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestClearThumbnailHandler(t *testing.T) {
	repo := sampleRepo()
	_, admin, _ := newTestHandlers(repo)

	rec := serve(admin.ClearThumbnailHandler, http.MethodPost, "/x", `{"thumbnailPath":"Movies/Classics/Film.jpg"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if _, ok := repo["Movies/Classics/Film.jpg"]; ok {
		t.Error("thumbnail should be deleted")
	}

	rec = serve(admin.ClearThumbnailHandler, http.MethodPost, "/x", `{"thumbnailPath":"Movies/Classics/Film.jpg"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("deleting a missing thumbnail: status %d, want 500", rec.Code)
	}
}

func TestBulkHandlers(t *testing.T) {
	repo := sampleRepo()
	_, admin, _ := newTestHandlers(repo)

	rec := serve(admin.BulkGenerateThumbnailsHandler, http.MethodPost, "/x", `{"timeMs":1000}`)
	var gen map[string]any
	json.Unmarshal(rec.Body.Bytes(), &gen)
	if rec.Code != http.StatusOK || gen["processed"] != float64(1) || gen["errors"] != float64(0) {
		t.Fatalf("bulk generate: status %d body %s", rec.Code, rec.Body)
	}

	rec = serve(admin.BulkClearThumbnailsHandler, http.MethodPost, "/x", "")
	var clr map[string]any
	json.Unmarshal(rec.Body.Bytes(), &clr)
	if rec.Code != http.StatusOK || clr["deleted"] != float64(2) {
		t.Fatalf("bulk clear: status %d body %s", rec.Code, rec.Body)
	}
}

func TestSearchMoviePosterHandlerReturnsEmptyArray(t *testing.T) {
	_, admin, _ := newTestHandlers(sampleRepo())
	rec := serve(admin.SearchMoviePosterHandler, http.MethodGet, "/x?movieTitle=Nothing", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("status %d body %q, want 200 []", rec.Code, rec.Body)
	}
}
