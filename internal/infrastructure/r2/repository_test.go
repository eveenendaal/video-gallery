package r2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// newTestRepository builds a StorageRepository against a fake R2 endpoint.
// Presigning is pure local SigV4 signing, so this needs no network access.
func newTestRepository() *StorageRepository {
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String("https://test-account.r2.cloudflarestorage.com"),
		Credentials:  credentials.NewStaticCredentialsProvider("test-access-key", "test-secret-key", ""),
	})
	return NewStorageRepository("test-bucket", client, s3.NewPresignClient(client))
}

func TestGetSignedURL(t *testing.T) {
	repo := newTestRepository()

	url, err := repo.GetSignedURL(context.Background(), "Category/Gallery/video.mp4", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(url, "test-bucket") {
		t.Errorf("expected signed URL to reference the bucket, got %q", url)
	}
	if !strings.Contains(url, "Category/Gallery/video.mp4") {
		t.Errorf("expected signed URL to reference the object path, got %q", url)
	}
	if !strings.Contains(url, "X-Amz-Signature=") {
		t.Errorf("expected signed URL to contain a SigV4 signature, got %q", url)
	}
	if !strings.Contains(url, "X-Amz-Expires=3600") {
		t.Errorf("expected signed URL to expire in 3600 seconds, got %q", url)
	}
}

func TestValidateLocalPath(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "video-gallery-thumbnails")

	if err := validateLocalPath(filepath.Join(tmpDir, "thumb.jpg")); err != nil {
		t.Errorf("expected path within temp dir to be valid, got error: %v", err)
	}

	if err := validateLocalPath("relative/path.jpg"); err == nil {
		t.Error("expected relative path to be rejected")
	}

	if err := validateLocalPath("/etc/passwd"); err == nil {
		t.Error("expected path outside temp dir to be rejected")
	}
}
