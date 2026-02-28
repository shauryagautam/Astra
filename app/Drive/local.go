package drive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalDriver implements the DiskContract for local file storage.
type LocalDriver struct {
	root string
}

// NewLocalDriver creates a new LocalDriver with the given root directory.
func NewLocalDriver(root string) *LocalDriver {
	return &LocalDriver{root: root}
}

func (l *LocalDriver) getPath(path string) string {
	return filepath.Join(l.root, path)
}

func (l *LocalDriver) Put(path string, contents []byte) error {
	fullPath := l.getPath(path)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}

	return os.WriteFile(fullPath, contents, 0644)
}

func (l *LocalDriver) PutStream(path string, reader io.Reader) error {
	fullPath := l.getPath(path)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("failed to write stream to %s: %w", path, err)
	}

	return nil
}

func (l *LocalDriver) Get(path string) ([]byte, error) {
	return os.ReadFile(l.getPath(path))
}

func (l *LocalDriver) Exists(path string) (bool, error) {
	_, err := os.Stat(l.getPath(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (l *LocalDriver) Delete(path string) error {
	return os.Remove(l.getPath(path))
}

func (l *LocalDriver) Url(path string) string {
	// For local storage, the URL depends on the application's URL and static file serving.
	// This is typically handled by a middleware or a separate static file server.
	return "/storage/" + path
}
