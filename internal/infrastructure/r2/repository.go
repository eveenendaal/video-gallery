package r2

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"video-gallery/internal/domain/gallery"
)

// StorageRepository is a Cloudflare R2 (S3-compatible) implementation of gallery.StorageRepository
type StorageRepository struct {
	bucketName    string
	client        *s3.Client
	presignClient *s3.PresignClient
}

// NewStorageRepository creates a new R2-backed StorageRepository.
func NewStorageRepository(bucketName string, client *s3.Client, presignClient *s3.PresignClient) *StorageRepository {
	return &StorageRepository{
		bucketName:    bucketName,
		client:        client,
		presignClient: presignClient,
	}
}

// ListObjects returns all objects in the bucket
func (r *StorageRepository) ListObjects(ctx context.Context) ([]gallery.StorageObject, error) {
	var objects []gallery.StorageObject

	paginator := s3.NewListObjectsV2Paginator(r.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(r.bucketName),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing bucket objects: %v", err)
		}
		for _, obj := range page.Contents {
			objects = append(objects, gallery.StorageObject{Name: aws.ToString(obj.Key)})
		}
	}
	return objects, nil
}

// GetSignedURL returns a time-limited presigned URL for the given object path
func (r *StorageRepository) GetSignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	req, err := r.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(path),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("error creating signed URL for %s: %v", path, err)
	}
	return req.URL, nil
}

// DeleteObject deletes the object at the given path from the bucket
func (r *StorageRepository) DeleteObject(ctx context.Context, path string) error {
	if _, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(path),
	}); err != nil {
		return fmt.Errorf("failed to delete object %s: %v", path, err)
	}
	return nil
}

// DownloadObject downloads the remote object to the local destination path.
// The destination must be an absolute path within the temp directory.
func (r *StorageRepository) DownloadObject(ctx context.Context, remotePath, localPath string) error {
	if err := gallery.ValidateWorkPath(localPath); err != nil {
		return err
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer f.Close()

	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return fmt.Errorf("GetObject(%q): %v", remotePath, err)
	}
	defer out.Body.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	return nil
}

// UploadObject uploads the local JPEG at localPath to the given object path
func (r *StorageRepository) UploadObject(ctx context.Context, localPath, remotePath string) error {
	if err := gallery.ValidateWorkPath(localPath); err != nil {
		return err
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("os.Open: %v", err)
	}
	defer f.Close()

	if _, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(remotePath),
		Body:        f,
		ContentType: aws.String("image/jpeg"),
	}); err != nil {
		return fmt.Errorf("PutObject: %v", err)
	}
	return nil
}
