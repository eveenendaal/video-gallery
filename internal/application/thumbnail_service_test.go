package application

import (
	"errors"
	"os"
	"slices"
	"testing"

	"video-gallery/internal/domain/gallery"
)

func newThumbnailTestService(repo *fakeRepo, proc *fakeProcessor) (*ThumbnailService, *GalleryService) {
	gallerySvc := NewGalleryService(repo, "secret")
	return NewThumbnailService(repo, proc, gallerySvc), gallerySvc
}

// assertNoTempFiles fails if any temp files were left behind in the work dir
func assertNoTempFiles(t *testing.T, before []os.DirEntry) {
	t.Helper()
	after, _ := os.ReadDir(gallery.WorkDir())
	if len(after) > len(before) {
		t.Errorf("temp files leaked: %d before, %d after", len(before), len(after))
	}
}

func TestGenerateThumbnail(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/Film.mp4")
	proc := &fakeProcessor{}
	svc, gallerySvc := newThumbnailTestService(repo, proc)
	before, _ := os.ReadDir(gallery.WorkDir())

	gallerySvc.GetVideos() // warm the cache
	var progress progressLog
	if err := svc.GenerateThumbnail("Movies/Classics/Film.mp4", 2500, progress.cb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "frame@2500 of content of Movies/Classics/Film.mp4"
	if got := repo.content("Movies/Classics/Film.jpg"); got != want {
		t.Errorf("uploaded thumbnail = %q, want %q", got, want)
	}
	if !slices.IsSorted(progress.steps) || progress.steps[len(progress.steps)-1] != 100 {
		t.Errorf("progress should increase to 100, got %v", progress.steps)
	}
	if v := gallerySvc.GetVideos(); v[0].Thumbnail == nil {
		t.Error("cache should be invalidated so the new thumbnail shows up")
	}
	assertNoTempFiles(t, before)
}

func TestGenerateThumbnailFailuresKeepExistingThumbnail(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeRepo, *fakeProcessor)
	}{
		{"download", func(r *fakeRepo, _ *fakeProcessor) { r.failOn["Movies/Classics/Film.mp4"] = errors.New("boom") }},
		{"extract", func(_ *fakeRepo, p *fakeProcessor) { p.extractErr = errors.New("boom") }},
		{"validate", func(_ *fakeRepo, p *fakeProcessor) { p.validateErr = errors.New("solid colour") }},
		{"upload", func(r *fakeRepo, _ *fakeProcessor) { r.failOn["Movies/Classics/Film.jpg"] = errors.New("boom") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo("Movies/Classics/Film.mp4", "Movies/Classics/Film.jpg")
			proc := &fakeProcessor{}
			tt.setup(repo, proc)
			svc, _ := newThumbnailTestService(repo, proc)
			before, _ := os.ReadDir(gallery.WorkDir())

			if err := svc.GenerateThumbnail("Movies/Classics/Film.mp4", 0, nil); err == nil {
				t.Fatal("expected an error")
			}
			if repo.content("Movies/Classics/Film.jpg") != "content of Movies/Classics/Film.jpg" {
				t.Error("existing thumbnail should be untouched when generation fails")
			}
			assertNoTempFiles(t, before)
		})
	}
}

func TestClearThumbnail(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/Film.mp4", "Movies/Classics/Film.jpg")
	svc, _ := newThumbnailTestService(repo, &fakeProcessor{})

	if err := svc.ClearThumbnail("Movies/Classics/Film.jpg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.has("Movies/Classics/Film.jpg") {
		t.Error("thumbnail should be deleted")
	}

	repo.failOn["Movies/Classics/Other.jpg"] = errors.New("boom")
	if err := svc.ClearThumbnail("Movies/Classics/Other.jpg"); err == nil {
		t.Error("expected delete error to propagate")
	}
}

func TestBulkGenerateThumbnails(t *testing.T) {
	newRepo := func() *fakeRepo {
		return newFakeRepo(
			"Movies/Classics/HasThumb.mp4",
			"Movies/Classics/HasThumb.png",
			"Movies/Classics/NoThumb.mp4",
			"Movies/Classics/Broken.mp4",
			"Movies/Classics/readme.txt",
		)
	}

	t.Run("missing only", func(t *testing.T) {
		repo := newRepo()
		repo.failOn["Movies/Classics/Broken.mp4"] = errors.New("boom")
		svc, _ := newThumbnailTestService(repo, &fakeProcessor{})

		processed, failed, err := svc.BulkGenerateThumbnails(1000, false)
		if err != nil || processed != 1 || failed != 1 {
			t.Fatalf("got processed=%d failed=%d err=%v, want 1, 1, nil", processed, failed, err)
		}
		if !repo.has("Movies/Classics/NoThumb.jpg") {
			t.Error("missing thumbnail should be generated")
		}
		if repo.has("Movies/Classics/HasThumb.jpg") {
			t.Error("video with an existing thumbnail should be skipped")
		}
	})

	t.Run("force", func(t *testing.T) {
		repo := newRepo()
		svc, _ := newThumbnailTestService(repo, &fakeProcessor{})

		processed, failed, err := svc.BulkGenerateThumbnails(1000, true)
		if err != nil || processed != 3 || failed != 0 {
			t.Fatalf("got processed=%d failed=%d err=%v, want 3, 0, nil", processed, failed, err)
		}
		if !repo.has("Movies/Classics/HasThumb.jpg") {
			t.Error("force should regenerate existing thumbnails")
		}
	})

	t.Run("list error", func(t *testing.T) {
		repo := newRepo()
		repo.listErr = errors.New("boom")
		svc, _ := newThumbnailTestService(repo, &fakeProcessor{})
		if _, _, err := svc.BulkGenerateThumbnails(1000, false); err == nil {
			t.Error("expected list error to propagate")
		}
	})
}

func TestBulkClearThumbnails(t *testing.T) {
	repo := newFakeRepo(
		"Movies/Classics/A.mp4",
		"Movies/Classics/A.jpg",
		"Movies/Classics/B.png",
		"Movies/Classics/C.jpeg",
		"cover.jpg", // outside the gallery layout: left alone
	)
	repo.failOn["Movies/Classics/C.jpeg"] = errors.New("boom")
	svc, _ := newThumbnailTestService(repo, &fakeProcessor{})

	deleted, err := svc.BulkClearThumbnails()
	if err != nil || deleted != 2 {
		t.Fatalf("got deleted=%d err=%v, want 2, nil", deleted, err)
	}
	if !repo.has("Movies/Classics/A.mp4") || !repo.has("cover.jpg") {
		t.Error("videos and non-gallery objects must not be deleted")
	}
	if repo.has("Movies/Classics/A.jpg") || repo.has("Movies/Classics/B.png") {
		t.Error("thumbnails should be deleted")
	}
}
