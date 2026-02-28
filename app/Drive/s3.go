package drive

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/shaurya/adonis/contracts"
)

// S3Config holds configuration for the S3 driver.
type S3Config struct {
	Region string
	Bucket string
	Key    string
	Secret string
}

// S3Driver implements the DiskContract for Amazon S3.
type S3Driver struct {
	client *s3.Client
	bucket string
}

// NewS3Driver creates a new S3 driver.
func NewS3Driver(ctx context.Context, cfg S3Config) (*S3Driver, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}

	client := s3.NewFromConfig(awsCfg)
	return &S3Driver{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Driver) Exists(path string) (bool, error) {
	_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *S3Driver) Get(path string) ([]byte, error) {
	resp, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *S3Driver) Put(path string, contents []byte) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
		Body:   bytes.NewReader(contents),
	})
	return err
}

func (s *S3Driver) PutStream(path string, resource io.Reader) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
		Body:   resource,
	})
	return err
}

func (s *S3Driver) Delete(path string) error {
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	return err
}

func (s *S3Driver) Url(path string) string {
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, path)
}

var _ contracts.DiskContract = (*S3Driver)(nil)
