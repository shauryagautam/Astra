package storage

import (
	"context"
	"net/http"
	"time"
)

// Storage defines the interface for file storage.
type Storage interface {
	Put(ctx context.Context, path string, content []byte) error
	Get(ctx context.Context, path string) ([]byte, error)
	Delete(ctx context.Context, path string) error
	URL(path string) (string, error)
	SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error)
	Exists(ctx context.Context, path string) (bool, error)
	Copy(ctx context.Context, src, dest string) error
	Move(ctx context.Context, src, dest string) error
}

// DetectMIME detects the MIME type of a byte slice.
func DetectMIME(content []byte) string {
	if len(content) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(content)
}
