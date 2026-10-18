package handlers

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eknkc/pug"

	"video-gallery/internal/domain/gallery"
)

func sampleCategories() []gallery.Category {
	thumb := "https://example.com/thumb.jpg"
	return []gallery.Category{
		{
			Name: "Home Movies",
			Galleries: []gallery.Gallery{
				{
					Name:     "Summer",
					Category: "Home Movies",
					Stub:     "/summer-stub",
					Videos: []gallery.Video{
						{Name: "Lake day", Url: "https://example.com/lake.mp4", Thumbnail: &thumb, VideoPath: "Home Movies/Summer/Lake day.mp4", ThumbnailPath: "Home Movies/Summer/Lake day.jpg"},
						{Name: "Fireworks", Url: "https://example.com/fireworks.mp4", VideoPath: "Home Movies/Summer/Fireworks.mp4"},
					},
				},
			},
		},
	}
}

func renderPug(t *testing.T, file string, data any) string {
	t.Helper()
	// TestMain runs the package's tests from the repository root.
	tpl, err := pug.CompileFile(filepath.Join(templateDir, file), pug.Options{})
	if err != nil {
		t.Fatalf("compile %s: %v", file, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute %s: %v", file, err)
	}
	return buf.String()
}

func assertContains(t *testing.T, html string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("rendered output missing %q", want)
		}
	}
}

func TestIndexTemplateRenders(t *testing.T) {
	html := renderPug(t, "index.pug", Index{Categories: sampleCategories()})
	assertContains(t, html,
		`href="/summer-stub"`,
		"Summer",
		"1 gallery",
		"2 videos",
		`src="https://example.com/thumb.jpg"`,
	)
}

func TestGalleryTemplateRenders(t *testing.T) {
	g := sampleCategories()[0].Galleries[0]
	html := renderPug(t, "gallery.pug", g)
	assertContains(t, html,
		"Home Movies",
		"2 videos",
		`href="https://example.com/lake.mp4"`,
		"No preview yet",
	)
}

func TestAdminTemplateRenders(t *testing.T) {
	html := renderPug(t, "admin.pug", Admin{Categories: sampleCategories(), SecretKey: "key"})
	assertContains(t, html,
		`id="videoTable"`,
		`name="Home Movies/Summer/Lake day.mp4"`,
		`class="thumbnail-mode"`,
		`class="time-input-control"`,
		`btn-clear`,
		`name="filterStatus"`,
		`const secretKey = 'key'`,
	)
	// Only the video with a thumbnail gets a clear button.
	if n := strings.Count(html, `class="vg-btn is-icon is-danger btn-clear"`); n != 1 {
		t.Errorf("expected 1 clear button, got %d", n)
	}
}
