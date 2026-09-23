package application

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestGetVideosPairsVideosWithThumbnails(t *testing.T) {
	repo := newFakeRepo(
		"Movies/Classics/Film.mp4",
		"Movies/Classics/Film.jpg",
		"Movies/Classics/NoThumb.mov",
		"Movies/Classics/notes.txt", // not media: ignored
		"Movies/loose.mp4",          // wrong depth: ignored
		"a/b/c/deep.mp4",            // wrong depth: ignored
	)
	svc := NewGalleryService(repo, "secret")

	videos := svc.GetVideos()
	if len(videos) != 2 {
		t.Fatalf("expected 2 videos, got %d: %+v", len(videos), videos)
	}

	film, noThumb := videos[0], videos[1]
	if film.Name != "Film" || film.Category != "Movies" || film.Gallery != "Classics" {
		t.Errorf("unexpected video metadata: %+v", film)
	}
	if film.Url != "https://signed.example/Movies/Classics/Film.mp4" || film.VideoPath != "Movies/Classics/Film.mp4" {
		t.Errorf("unexpected video URL/path: %+v", film)
	}
	if film.Thumbnail == nil || *film.Thumbnail != "https://signed.example/Movies/Classics/Film.jpg" ||
		film.ThumbnailPath != "Movies/Classics/Film.jpg" {
		t.Errorf("expected thumbnail to be paired, got %+v", film)
	}
	if noThumb.Name != "NoThumb" || noThumb.Thumbnail != nil || noThumb.ThumbnailPath != "" {
		t.Errorf("expected video without thumbnail, got %+v", noThumb)
	}
}

func TestGetVideosKeepsSameNamedVideosInDifferentGalleriesApart(t *testing.T) {
	repo := newFakeRepo(
		"Home/Alice/Birthday.mp4",
		"Home/Alice/Birthday.jpg",
		"Home/Bob/Birthday.mp4",
	)
	videos := NewGalleryService(repo, "secret").GetVideos()
	if len(videos) != 2 {
		t.Fatalf("expected 2 distinct videos, got %d: %+v", len(videos), videos)
	}
	for _, v := range videos {
		switch v.Gallery {
		case "Alice":
			if v.Thumbnail == nil {
				t.Error("Alice's video should have its thumbnail")
			}
		case "Bob":
			if v.Thumbnail != nil {
				t.Error("Bob's video must not pick up Alice's thumbnail")
			}
		default:
			t.Errorf("unexpected gallery %q", v.Gallery)
		}
	}
}

func TestGetVideosSkipsObjectsThatFailToSign(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/A.mp4", "Movies/Classics/B.mp4")
	repo.failOn["Movies/Classics/B.mp4"] = errors.New("boom")

	videos := NewGalleryService(repo, "secret").GetVideos()
	if len(videos) != 1 || videos[0].Name != "A" {
		t.Fatalf("expected only A, got %+v", videos)
	}
}

func TestGetVideosCachesUntilInvalidated(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/A.mp4")
	svc := NewGalleryService(repo, "secret")

	svc.GetVideos()
	svc.GetVideos()
	if repo.listCalls != 1 {
		t.Fatalf("expected cached listing, got %d list calls", repo.listCalls)
	}

	svc.InvalidateCache()
	svc.GetVideos()
	if repo.listCalls != 2 {
		t.Fatalf("expected re-listing after invalidation, got %d list calls", repo.listCalls)
	}
}

func TestGetVideosListErrorIsNotCached(t *testing.T) {
	repo := newFakeRepo("Movies/Classics/A.mp4")
	repo.listErr = errors.New("unavailable")
	svc := NewGalleryService(repo, "secret")

	if videos := svc.GetVideos(); videos == nil || len(videos) != 0 {
		t.Fatalf("expected empty non-nil slice on error, got %#v", videos)
	}

	repo.listErr = nil
	if videos := svc.GetVideos(); len(videos) != 1 {
		t.Fatalf("expected recovery after error, got %+v", videos)
	}
}

func TestGetGalleriesAndCategories(t *testing.T) {
	repo := newFakeRepo(
		"Movies/Sequels/Part 10.mp4",
		"Movies/Sequels/Part 2.mp4",
		"Home/Bob/Clip.mp4",
		"Home/Alice/Clip.mp4",
	)
	svc := NewGalleryService(repo, "secret")

	galleries := svc.GetGalleries()
	var names []string
	for _, g := range galleries {
		names = append(names, g.Name)
	}
	if !slices.Equal(names, []string{"Alice", "Bob", "Sequels"}) {
		t.Errorf("galleries not sorted by name: %v", names)
	}

	sequels := galleries[2]
	if sequels.Category != "Movies" || len(sequels.Videos) != 2 ||
		sequels.Videos[0].Name != "Part 2" || sequels.Videos[1].Name != "Part 10" {
		t.Errorf("unexpected Sequels gallery: %+v", sequels)
	}

	categories := svc.GetCategories()
	if len(categories) != 2 || categories[0].Name != "Home" || categories[1].Name != "Movies" {
		t.Fatalf("unexpected categories: %+v", categories)
	}
	if categories[0].Stub != "Home" || len(categories[0].Galleries) != 2 {
		t.Errorf("unexpected Home category: %+v", categories[0])
	}
}

func TestGalleryStubs(t *testing.T) {
	repo := newFakeRepo("Home/Alice/Clip.mp4", "Home/Bob/Clip.mp4")
	svc := NewGalleryService(repo, "secret")

	galleries := svc.GetGalleries()
	alice, bob := galleries[0], galleries[1]

	if !strings.HasPrefix(alice.Stub, "/gallery/") || len(alice.Stub) != len("/gallery/")+16 {
		t.Errorf("unexpected stub format: %q", alice.Stub)
	}
	if alice.Stub == bob.Stub {
		t.Error("different galleries must get different stubs")
	}
	// Stubs are shared as links, so they must be stable for a given name and key.
	if alice.Stub != "/gallery/"+NewGalleryService(nil, "secret").galleryHash("Alice") {
		t.Error("stub should be derived from the gallery name and secret key")
	}
	if alice.Stub == "/gallery/"+NewGalleryService(nil, "other").galleryHash("Alice") {
		t.Error("stub must depend on the secret key")
	}

	got, err := svc.GetGallery(bob.Stub)
	if err != nil || got.Name != "Bob" {
		t.Errorf("GetGallery(%q) = %+v, %v", bob.Stub, got, err)
	}
	if _, err := svc.GetGallery("/gallery/unknown"); err == nil {
		t.Error("expected error for unknown stub")
	}
}

func TestNaturalLess(t *testing.T) {
	sorted := []string{
		"",
		"Episode 1",
		"Episode 2",
		"Episode 10",
		"Episode 010b",
		"Episode 99999999999999999999999",
		"Episode 100000000000000000000000",
		"a",
		"b",
	}
	shuffled := slices.Clone(sorted)
	slices.Reverse(shuffled)
	sort.Slice(shuffled, func(i, j int) bool { return naturalLess(shuffled[i], shuffled[j]) })
	if !slices.Equal(shuffled, sorted) {
		t.Errorf("natural sort mismatch:\n got  %q\n want %q", shuffled, sorted)
	}

	if naturalLess("Film 2", "Film2") || naturalLess("Film2", "Film 2") {
		t.Error("whitespace should be ignored when comparing")
	}
	if naturalLess("x", "x") {
		t.Error("naturalLess must be irreflexive")
	}
}
