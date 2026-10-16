package s3

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	errs "github.com/vasmaae/distributed-computing-and-applications/files/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/service/file"
)

var _ file.Repository = (*Repository)(nil)

type Repository struct {
	client *awss3.Client
	bucket string
}

func NewRepository(client *awss3.Client, bucket string) *Repository {
	return &Repository{
		client: client,
		bucket: bucket,
	}
}

func (r Repository) Upload(ctx context.Context, key string, src io.Reader) error {
	_, err := r.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        src,
		ContentType: aws.String("text/csv; charset=utf-8"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file %q: %w", key, err)
	}

	return nil
}

func (r Repository) Get(ctx context.Context, key string, dst io.Writer) error {
	out, err := r.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var missing *types.NoSuchKey
		if errors.As(err, &missing) {
			return errs.ErrNotFound
		}
		return fmt.Errorf("failed to get file %q: %w", key, err)
	}

	defer out.Body.Close() //nolint:errcheck

	if _, err := io.Copy(dst, out.Body); err != nil {
		return fmt.Errorf("failed to copy file %q: %w", key, err)
	}

	return nil
}
