package ffmpeg

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFormatTimestamp(t *testing.T) {
	tests := map[int]string{
		0:          "00:00:00.000",
		1000:       "00:00:01.000",
		61_500:     "00:01:01.500",
		3_723_004:  "01:02:03.004",
		36_000_000: "10:00:00.000",
	}
	for ms, want := range tests {
		if got := formatTimestamp(ms); got != want {
			t.Errorf("formatTimestamp(%d) = %q, want %q", ms, got, want)
		}
	}
}

// writePNG writes a w×h PNG whose pixels are chosen by colorAt
func writePNG(t *testing.T, w, h int, colorAt func(x, y int) color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, colorAt(x, y))
		}
	}
	path := filepath.Join(t.TempDir(), "img.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateImage(t *testing.T) {
	p := NewProcessor()

	solid := writePNG(t, 100, 100, func(int, int) color.Color { return color.Black })
	if err := p.ValidateImage(solid); err == nil {
		t.Error("solid-colour image should be rejected")
	}

	gradient := writePNG(t, 100, 100, func(x, y int) color.Color {
		return color.RGBA{uint8(x * 2), uint8(y * 2), 128, 255}
	})
	if err := p.ValidateImage(gradient); err != nil {
		t.Errorf("varied image should be accepted: %v", err)
	}

	tiny := writePNG(t, 3, 3, func(x, _ int) color.Color {
		return color.Gray{uint8(x * 100)}
	})
	if err := p.ValidateImage(tiny); err != nil {
		t.Errorf("small varied image should be accepted: %v", err)
	}

	if err := p.ValidateImage(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("missing file should be rejected")
	}

	notImage := filepath.Join(t.TempDir(), "bad.png")
	os.WriteFile(notImage, []byte("not an image"), 0o644)
	if err := p.ValidateImage(notImage); err == nil {
		t.Error("undecodable file should be rejected")
	}
}

func TestExtractFrame(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "test.mp4")
	gen := exec.Command("ffmpeg", "-f", "lavfi", "-i", "testsrc=duration=2:size=64x64:rate=10", "-y", video)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test video: %v\n%s", err, out)
	}

	thumb := filepath.Join(dir, "thumb.jpg")
	p := NewProcessor()
	if err := p.ExtractFrame(video, thumb, 1000); err != nil {
		t.Fatalf("ExtractFrame failed: %v", err)
	}
	if err := p.ValidateImage(thumb); err != nil {
		t.Errorf("extracted frame should be a valid image: %v", err)
	}
}
