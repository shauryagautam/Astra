package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "astra-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	s := NewLocalStorage(tempDir)
	ctx := context.Background()

	t.Run("Put and Get", func(t *testing.T) {
		path := "test/hello.txt"
		content := []byte("hello astra")

		err := s.Put(ctx, path, content)
		require.NoError(t, err)

		got, err := s.Get(ctx, path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("Exists", func(t *testing.T) {
		path := "exists.txt"
		s.Put(ctx, path, []byte("data"))

		exists, err := s.Exists(ctx, path)
		require.NoError(t, err)
		assert.True(t, exists)

		exists, err = s.Exists(ctx, "nonexistent.txt")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		path := "delete-me.txt"
		s.Put(ctx, path, []byte("bye"))

		err := s.Delete(ctx, path)
		require.NoError(t, err)

		exists, _ := s.Exists(ctx, path)
		assert.False(t, exists)
	})

	t.Run("URL", func(t *testing.T) {
		url, err := s.URL("foo/bar.jpg")
		require.NoError(t, err)
		assert.Equal(t, "/storage/foo/bar.jpg", url)
	})
}

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected string
	}{
		{"Empty", []byte{}, "application/octet-stream"},
		{"Text", []byte("hello world"), "text/plain; charset=utf-8"},
		{"PNG", []byte("\x89PNG\r\n\x1a\n"), "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DetectMIME(tt.content))
		})
	}
}

// MockStorage for other packages to use in testing
type MockStorage struct {
	Files map[string][]byte
}

func (m *MockStorage) Put(ctx context.Context, path string, content []byte) error {
	m.Files[path] = content
	return nil
}

func (m *MockStorage) Get(ctx context.Context, path string) ([]byte, error) {
	if f, ok := m.Files[path]; ok {
		return f, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	delete(m.Files, path)
	return nil
}

func (m *MockStorage) URL(path string) (string, error) {
	return "http://mock/" + path, nil
}

func (m *MockStorage) SignedURL(ctx context.Context, path string, d time.Duration) (string, error) {
	return m.URL(path)
}

func (m *MockStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, ok := m.Files[path]
	return ok, nil
}
