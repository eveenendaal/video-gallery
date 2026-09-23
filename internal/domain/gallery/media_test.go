package gallery

import (
	"path/filepath"
	"testing"
)

func TestIsVideoAndIsImage(t *testing.T) {
	tests := []struct {
		name           string
		video, isImage bool
	}{
		{"movie.mp4", true, false},
		{"movie.MOV", true, false},
		{"a/b/clip.webm", true, false},
		{"poster.jpg", false, true},
		{"poster.JPEG", false, true},
		{"poster.png", false, true},
		{"notes.txt", false, false},
		{"noext", false, false},
	}
	for _, tt := range tests {
		if got := IsVideo(tt.name); got != tt.video {
			t.Errorf("IsVideo(%q) = %v, want %v", tt.name, got, tt.video)
		}
		if got := IsImage(tt.name); got != tt.isImage {
			t.Errorf("IsImage(%q) = %v, want %v", tt.name, got, tt.isImage)
		}
	}
}

func TestParseObjectPath(t *testing.T) {
	p, ok := ParseObjectPath("Movies/Classics/Film 1.mp4")
	if !ok || p != (ObjectPath{Category: "Movies", Gallery: "Classics", File: "Film 1.mp4"}) {
		t.Fatalf("unexpected parse result: %+v, %v", p, ok)
	}

	for _, key := range []string{
		"",
		"file.mp4",
		"Movies/file.mp4",
		"Movies/Classics/",
		"/Classics/file.mp4",
		"Movies//file.mp4",
		"a/b/c/d.mp4",
	} {
		if _, ok := ParseObjectPath(key); ok {
			t.Errorf("ParseObjectPath(%q) should fail", key)
		}
	}
}

func TestThumbnailPathFor(t *testing.T) {
	tests := map[string]string{
		"Movies/Classics/Film.mp4":     "Movies/Classics/Film.jpg",
		"Movies/Classics/Film.v2.mov":  "Movies/Classics/Film.v2.jpg",
		"Movies/Classics/Poster.png":   "Movies/Classics/Poster.jpg",
		"Movies/Classics/no-extension": "Movies/Classics/no-extension.jpg",
	}
	for in, want := range tests {
		if got := ThumbnailPathFor(in); got != want {
			t.Errorf("ThumbnailPathFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateWorkPath(t *testing.T) {
	if err := ValidateWorkPath(filepath.Join(WorkDir(), "thumb.jpg")); err != nil {
		t.Errorf("expected path within work dir to be valid, got: %v", err)
	}

	for _, path := range []string{
		"relative/path.jpg",
		"/etc/passwd",
		WorkDir(),
		filepath.Join(WorkDir(), "..", "escape.jpg"),
		WorkDir() + "-sibling/file.jpg",
	} {
		if err := ValidateWorkPath(path); err == nil {
			t.Errorf("expected %q to be rejected", path)
		}
	}
}
