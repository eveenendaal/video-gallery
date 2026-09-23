package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"video-gallery/internal/domain/gallery"
)

// fakeRepo is an in-memory gallery.StorageRepository
type fakeRepo struct {
	mu        sync.Mutex
	objects   map[string][]byte
	listCalls int
	listErr   error
	failOn    map[string]error // per-path errors for Download/Upload/Delete
}

func newFakeRepo(keys ...string) *fakeRepo {
	r := &fakeRepo{objects: map[string][]byte{}, failOn: map[string]error{}}
	for _, k := range keys {
		r.objects[k] = []byte("content of " + k)
	}
	return r
}

func (r *fakeRepo) ListObjects(context.Context) ([]gallery.StorageObject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listCalls++
	if r.listErr != nil {
		return nil, r.listErr
	}
	var objs []gallery.StorageObject
	for k := range r.objects {
		objs = append(objs, gallery.StorageObject{Name: k})
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].Name < objs[j].Name })
	return objs, nil
}

func (r *fakeRepo) GetSignedURL(_ context.Context, path string, _ time.Duration) (string, error) {
	if err := r.failOn[path]; err != nil {
		return "", err
	}
	return "https://signed.example/" + path, nil
}

func (r *fakeRepo) DeleteObject(_ context.Context, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failOn[path]; err != nil {
		return err
	}
	delete(r.objects, path)
	return nil
}

func (r *fakeRepo) DownloadObject(_ context.Context, remotePath, localPath string) error {
	if err := gallery.ValidateWorkPath(localPath); err != nil {
		return err
	}
	r.mu.Lock()
	data, ok := r.objects[remotePath]
	err := r.failOn[remotePath]
	r.mu.Unlock()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("object %q not found", remotePath)
	}
	return os.WriteFile(localPath, data, 0o644)
}

func (r *fakeRepo) UploadObject(_ context.Context, localPath, remotePath string) error {
	if err := gallery.ValidateWorkPath(localPath); err != nil {
		return err
	}
	if err := r.failOn[remotePath]; err != nil {
		return err
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.objects[remotePath] = data
	return nil
}

func (r *fakeRepo) has(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.objects[path]
	return ok
}

func (r *fakeRepo) content(path string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.objects[path])
}

// fakeProcessor writes a marker "frame" and can be told to fail
type fakeProcessor struct {
	extractErr  error
	validateErr error
	extracted   []string // video contents seen by ExtractFrame
}

func (p *fakeProcessor) ExtractFrame(videoPath, thumbnailPath string, timeMs int) error {
	if p.extractErr != nil {
		return p.extractErr
	}
	data, err := os.ReadFile(videoPath)
	if err != nil {
		return err
	}
	p.extracted = append(p.extracted, string(data))
	return os.WriteFile(thumbnailPath, fmt.Appendf(nil, "frame@%d of %s", timeMs, data), 0o644)
}

func (p *fakeProcessor) ValidateImage(string) error { return p.validateErr }

// fakePosterClient serves canned TMDb results
type fakePosterClient struct {
	search      []gallery.MovieResult
	searchErr   error
	movies      map[int]gallery.MovieResult
	searchedFor string
	downloaded  string
}

func (c *fakePosterClient) SearchMovies(_ context.Context, title string) ([]gallery.MovieResult, error) {
	c.searchedFor = title
	return c.search, c.searchErr
}

func (c *fakePosterClient) GetMovie(_ context.Context, id int) (gallery.MovieResult, error) {
	m, ok := c.movies[id]
	if !ok {
		return gallery.MovieResult{}, errors.New("not found")
	}
	return m, nil
}

func (c *fakePosterClient) DownloadImage(_ context.Context, imageURL, destPath string) error {
	c.downloaded = imageURL
	return os.WriteFile(destPath, []byte("poster "+imageURL), 0o644)
}

func strPtr(s string) *string { return &s }

// progressLog records progress callbacks
type progressLog struct{ steps []int }

func (l *progressLog) cb(_ string, progress int) { l.steps = append(l.steps, progress) }
