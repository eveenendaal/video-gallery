package gcs

import (
	"context"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func TestLocalPathsMustBeInWorkDir(t *testing.T) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	repo := NewStorageRepository("test-bucket", client)

	if err := repo.DownloadObject(ctx, "A/B/c.mp4", "/etc/passwd"); err == nil {
		t.Error("download outside the work dir should be rejected")
	}
	if err := repo.UploadObject(ctx, "/etc/passwd", "A/B/c.jpg"); err == nil {
		t.Error("upload from outside the work dir should be rejected")
	}
}
