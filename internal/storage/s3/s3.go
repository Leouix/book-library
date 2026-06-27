package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3svc "github.com/aws/aws-sdk-go-v2/service/s3"
)

type FileStorage interface {
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
}

type S3Storage struct {
	client *s3svc.Client
	bucket string
}

func NewClient(ctx context.Context, keyID, appKey, region, endpoint string) (*s3svc.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(keyID, appKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}

	client := s3svc.NewFromConfig(cfg, func(o *s3svc.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return client, nil
}

func NewStorage(client *s3svc.Client, bucket string) *S3Storage {
	return &S3Storage{client: client, bucket: bucket}
}

func (s *S3Storage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3svc.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 put object: %w", err)
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3svc.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return output.Body, nil
}
