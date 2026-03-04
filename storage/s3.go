package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage implements the Storage interface for S3-compatible APIs.
type S3Storage struct {
	client *s3.Client
	config config.StorageConfig
}

// NewS3Storage creates a new S3Storage.
func NewS3Storage(ctx context.Context, cfg config.StorageConfig) (*S3Storage, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.S3Endpoint != "" {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               cfg.S3Endpoint,
				SigningRegion:     cfg.S3Region,
				HostnameImmutable: cfg.S3ForcePathStyle,
			}, nil
		}
		// returning EndpointNotFoundError will allow the service to fall back to its default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePathStyle
	})

	return &S3Storage{
		client: client,
		config: cfg,
	}, nil
}

// Put writes a file to S3.
func (s *S3Storage) Put(ctx context.Context, path string, content []byte) error {
	mime := DetectMIME(content)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.S3Bucket),
		Key:         aws.String(path),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(mime),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3: %w", err)
	}
	return nil
}

// Get reads a file from S3.
func (s *S3Storage) Get(ctx context.Context, path string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from s3: %w", err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}

// Delete removes a file from S3.
func (s *S3Storage) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from s3: %w", err)
	}
	return nil
}

// URL returns the public URL for the file.
func (s *S3Storage) URL(path string) (string, error) {
	// In a real implementation, you would generate a presigned URL or use a CDN domain
	// For simplicity, we construct a basic S3 URL
	if s.config.S3Endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", s.config.S3Endpoint, s.config.S3Bucket, path), nil
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.config.S3Bucket, s.config.S3Region, path), nil
}

// SignedURL returns a presigned URL for the file.
func (s *S3Storage) SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	pc := s3.NewPresignClient(s.client)
	res, err := pc.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(path),
	}, s3.WithPresignExpires(expiresIn))

	if err != nil {
		return "", fmt.Errorf("failed to presign url: %w", err)
	}
	return res.URL, nil
}

// Exists checks if an object exists in S3.
func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		var nf *types.NotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nf) || errors.As(err, &nsk) {
			return false, nil
		}
		// Also check for 404 response code if types don't match exactly
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence in s3: %w", err)
	}
	return true, nil
}
