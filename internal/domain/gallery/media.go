package gallery

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	videoExtensions = []string{".mp4", ".m4v", ".webm", ".mov", ".avi"}
	imageExtensions = []string{".jpg", ".jpeg", ".png"}
)

// IsVideo reports whether name has a supported video extension (case-insensitive)
func IsVideo(name string) bool {
	return slices.Contains(videoExtensions, strings.ToLower(filepath.Ext(name)))
}

// IsImage reports whether name has a supported thumbnail image extension (case-insensitive)
func IsImage(name string) bool {
	return slices.Contains(imageExtensions, strings.ToLower(filepath.Ext(name)))
}

// ObjectPath is a storage object key split into its "Category/Gallery/File" parts
type ObjectPath struct {
	Category string
	Gallery  string
	File     string
}

// ParseObjectPath splits a storage object key of the form "Category/Gallery/File".
// It returns false for keys at any other depth or with an empty segment.
func ParseObjectPath(key string) (ObjectPath, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ObjectPath{}, false
	}
	return ObjectPath{Category: parts[0], Gallery: parts[1], File: parts[2]}, true
}

// StripExt returns path without its final extension
func StripExt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path))
}

// ThumbnailPathFor returns the storage key a generated thumbnail for videoPath is written to
func ThumbnailPathFor(videoPath string) string {
	return StripExt(videoPath) + ".jpg"
}

// WorkDir is the local scratch directory used for temporary video and image files.
// Repositories only read from or write to local paths inside it.
func WorkDir() string {
	return filepath.Join(os.TempDir(), "video-gallery-thumbnails")
}

// ValidateWorkPath ensures path is absolute and inside WorkDir
func ValidateWorkPath(path string) error {
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("path must be absolute: %s", path)
	}
	if !strings.HasPrefix(cleanPath, WorkDir()+string(os.PathSeparator)) {
		return fmt.Errorf("invalid path: must be within temp directory")
	}
	return nil
}
