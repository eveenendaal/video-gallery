package r2

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"video-gallery/internal/domain/gallery"

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

func TestLocalPathsMustBeInWorkDir(t *testing.T) {
	repo := newTestRepository()
	ctx := context.Background()

	if err := repo.DownloadObject(ctx, "A/B/c.mp4", "/etc/passwd"); err == nil {
		t.Error("download outside the work dir should be rejected")
	}
	if err := repo.UploadObject(ctx, "/etc/passwd", "A/B/c.jpg"); err == nil {
		t.Error("upload from outside the work dir should be rejected")
	}
}

// fakeS3 is a minimal path-style S3 server holding one bucket in memory
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := strings.TrimPrefix(r.URL.Path, "/test-bucket/")
	switch {
	case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
		var b strings.Builder
		b.WriteString(`<ListBucketResult><Name>test-bucket</Name><IsTruncated>false</IsTruncated>`)
		for k := range f.objects {
			fmt.Fprintf(&b, "<Contents><Key>%s</Key></Contents>", k)
		}
		b.WriteString(`</ListBucketResult>`)
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, b.String())
	case r.Method == http.MethodGet:
		data, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `<Error><Code>NoSuchKey</Code></Error>`)
			return
		}
		w.Write(data)
	case r.Method == http.MethodPut:
		data, _ := io.ReadAll(r.Body)
		f.objects[key] = data
		f.types[key] = r.Header.Get("Content-Type")
	case r.Method == http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestRoundTripAgainstFakeS3(t *testing.T) {
	fake := &fakeS3{objects: map[string][]byte{"A/B/video.mp4": []byte("video bytes")}, types: map[string]string{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(srv.URL),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("id", "secret", ""),
	})
	repo := NewStorageRepository("test-bucket", client, s3.NewPresignClient(client))
	ctx := context.Background()

	if err := os.MkdirAll(gallery.WorkDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	local, err := os.CreateTemp(gallery.WorkDir(), "r2-test-*")
	if err != nil {
		t.Fatal(err)
	}
	local.Close()
	defer os.Remove(local.Name())

	if err := repo.DownloadObject(ctx, "A/B/video.mp4", local.Name()); err != nil {
		t.Fatalf("download: %v", err)
	}
	if data, _ := os.ReadFile(local.Name()); string(data) != "video bytes" {
		t.Errorf("downloaded %q", data)
	}
	if err := repo.DownloadObject(ctx, "A/B/missing.mp4", local.Name()); err == nil {
		t.Error("expected error downloading a missing object")
	}

	os.WriteFile(local.Name(), []byte("jpeg bytes"), 0o644)
	if err := repo.UploadObject(ctx, local.Name(), "A/B/video.jpg"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if string(fake.objects["A/B/video.jpg"]) != "jpeg bytes" || fake.types["A/B/video.jpg"] != "image/jpeg" {
		t.Errorf("unexpected uploaded object %q (%s)", fake.objects["A/B/video.jpg"], fake.types["A/B/video.jpg"])
	}

	objects, err := repo.ListObjects(ctx)
	if err != nil || len(objects) != 2 {
		t.Fatalf("list: got %+v, %v", objects, err)
	}

	if err := repo.DeleteObject(ctx, "A/B/video.jpg"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := fake.objects["A/B/video.jpg"]; ok {
		t.Error("object should be deleted")
	}
}
