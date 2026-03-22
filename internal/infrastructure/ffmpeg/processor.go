package ffmpeg

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// colorDifferenceThreshold is the minimum per-channel difference between two
	// pixels before they are considered different (accounts for JPEG compression
	// artifacts).
	colorDifferenceThreshold = uint32(256) // ~1 unit in 8-bit color
)

// Processor is an FFmpeg-backed implementation of gallery.VideoProcessor
type Processor struct{}

// NewProcessor creates a new FFmpeg Processor
func NewProcessor() *Processor {
	return &Processor{}
}

// ExtractFrame extracts a single video frame at timeMs milliseconds into the video
// and saves it as a JPEG image at thumbnailPath.
func (p *Processor) ExtractFrame(videoPath, thumbnailPath string, timeMs int) error {
	if err := checkFFmpeg(); err != nil {
		return fmt.Errorf("FFmpeg is required but not found: %v", err)
	}

	totalSeconds := timeMs / 1000
	milliseconds := timeMs % 1000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	timeStr := fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)

	cmd := exec.Command(
		"ffmpeg",
		"-ss", timeStr,
		"-i", videoPath,
		"-vf", "thumbnail",
		"-frames:v", "1",
		"-q:v", "2",
		"-y",
		thumbnailPath,
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %v, stderr: %s", err, stderr.String())
	}
	return nil
}

// ValidateImage checks that the image at imagePath is not a solid-colour frame
// (which would indicate that FFmpeg captured a blank section of the video).
func (p *Processor) ValidateImage(imagePath string) error {
	cleanPath := filepath.Clean(imagePath)

	f, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %v", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	sampleSize := 10
	stepX := width / sampleSize
	stepY := height / sampleSize
	if stepX == 0 {
		stepX = 1
	}
	if stepY == 0 {
		stepY = 1
	}

	firstColor := img.At(bounds.Min.X, bounds.Min.Y)
	r1, g1, b1, a1 := firstColor.RGBA()

	differentPixels := 0
	totalSamples := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			totalSamples++
			r2, g2, b2, a2 := img.At(x, y).RGBA()
			if absDiff(r1, r2) > colorDifferenceThreshold ||
				absDiff(g1, g2) > colorDifferenceThreshold ||
				absDiff(b1, b2) > colorDifferenceThreshold ||
				absDiff(a1, a2) > colorDifferenceThreshold {
				differentPixels++
			}
		}
	}

	if totalSamples > 0 && float64(differentPixels)/float64(totalSamples) < 0.01 {
		return fmt.Errorf("image appears to be a solid colour (%d/%d sampled pixels differ)",
			differentPixels, totalSamples)
	}
	return nil
}

func checkFFmpeg() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg not found or not working: %v", err)
	}
	return nil
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
