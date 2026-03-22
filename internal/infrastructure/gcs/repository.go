package gcs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"video-gallery/internal/domain/gallery"
)

// StorageRepository is a Google Cloud Storage implementation of gallery.StorageRepository
type StorageRepository struct {
	bucketName string
	client     *storage.Client
}

// NewStorageRepository creates a new GCS-backed StorageRepository.
// The caller must ensure Close is called when the repository is no longer needed.
func NewStorageRepository(bucketName string, client *storage.Client) *StorageRepository {
	return &StorageRepository{
		bucketName: bucketName,
		client:     client,
	}
}

// ListObjects returns all objects in the bucket
func (r *StorageRepository) ListObjects(ctx context.Context) ([]gallery.StorageObject, error) {
	bucket := r.client.Bucket(r.bucketName)
	it := bucket.Objects(ctx, nil)

	var objects []gallery.StorageObject
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error iterating bucket objects: %v", err)
		}
		objects = append(objects, gallery.StorageObject{Name: obj.Name})
	}
	return objects, nil
}

// GetSignedURL returns a time-limited signed URL for the given object path
func (r *StorageRepository) GetSignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	bucket := r.client.Bucket(r.bucketName)
	url, err := bucket.SignedURL(path, &storage.SignedURLOptions{
		Expires: time.Now().Add(expiry),
		Method:  "GET",
	})
	if err != nil {
		return "", fmt.Errorf("error creating signed URL for %s: %v", path, err)
	}
	return url, nil
}

// DeleteObject deletes the object at the given path from the bucket
func (r *StorageRepository) DeleteObject(ctx context.Context, path string) error {
	bucket := r.client.Bucket(r.bucketName)
	if err := bucket.Object(path).Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object %s: %v", path, err)
	}
	return nil
}

// DownloadObject downloads the remote object to the local destination path.
// The destination must be an absolute path within the temp directory.
func (r *StorageRepository) DownloadObject(ctx context.Context, remotePath, localPath string) error {
	if err := validateLocalPath(localPath); err != nil {
		return err
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer f.Close()

	// Only allow GCS object paths, not arbitrary HTTP URLs
	if strings.HasPrefix(remotePath, "http") {
		return fmt.Errorf("HTTP source paths are not allowed")
	}

	cleanRemote := strings.TrimPrefix(remotePath, "/")
	reader, err := r.client.Bucket(r.bucketName).Object(cleanRemote).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", cleanRemote, err)
	}
	defer reader.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	return nil
}

// UploadObject uploads the local file at srcPath to the given destination object path
func (r *StorageRepository) UploadObject(ctx context.Context, localPath, remotePath string) error {
	if err := validateLocalPath(localPath); err != nil {
		return err
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("os.ReadFile: %v", err)
	}

	// Strip query strings and leading slashes from the destination
	dst := strings.TrimPrefix(remotePath, "/")
	if idx := strings.Index(dst, "?"); idx != -1 {
		dst = dst[:idx]
	}

	writer := r.client.Bucket(r.bucketName).Object(dst).NewWriter(ctx)
	writer.ContentType = "image/jpeg"

	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("writer.Write: %v", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("writer.Close: %v", err)
	}
	return nil
}

// validateLocalPath ensures a path is absolute and within the designated temp directory
func validateLocalPath(path string) error {
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("path must be absolute: %s", path)
	}
	expectedDir := filepath.Clean(filepath.Join(os.TempDir(), "video-gallery-thumbnails"))
	if !strings.HasPrefix(cleanPath, expectedDir) {
		return fmt.Errorf("invalid path: must be within temp directory")
	}
	return nil
}
