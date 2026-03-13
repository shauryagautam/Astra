package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage implements the Storage interface for the local filesystem.
type LocalStorage struct {
	rootDir string
}

// NewLocalStorage creates a new LocalStorage.
func NewLocalStorage(rootDir string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir}
}

// Put writes a file to the local filesystem.
func (s *LocalStorage) Put(ctx context.Context, path string, content []byte) error {
	fullPath, err := s.securePath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(fullPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func (s *LocalStorage) securePath(path string) (string, error) {
	path = filepath.Clean(path)
	fullPath := filepath.Join(s.rootDir, path)
	rel, err := filepath.Rel(s.rootDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path traversal attempt: %s", path)
	}
	return fullPath, nil
}

// Get reads a file from the local filesystem.
func (s *LocalStorage) Get(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := s.securePath(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath) // #nosec G304 -- path validated by securePath
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log but don't override read error
		}
	}()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// Delete removes a file from the local filesystem.
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath, err := s.securePath(path)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

// URL returns a relative URL for the file.
func (s *LocalStorage) URL(path string) (string, error) {
	return "/storage/" + path, nil
}

// SignedURL returns a relative URL for the file (local storage doesn't presign).
func (s *LocalStorage) SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	return s.URL(path)
}

// Exists checks if a file exists on the local filesystem.
func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath, err := s.securePath(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check file existence: %w", err)
}

// Copy copies a file on the local filesystem.
func (s *LocalStorage) Copy(ctx context.Context, src, dest string) error {
	srcPath, err := s.securePath(src)
	if err != nil {
		return err
	}
	destPath, err := s.securePath(dest)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	return nil
}

// Move moves a file on the local filesystem.
func (s *LocalStorage) Move(ctx context.Context, src, dest string) error {
	srcPath, err := s.securePath(src)
	if err != nil {
		return err
	}
	destPath, err := s.securePath(dest)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		// Fallback to copy/delete if rename fails (e.g. cross-device)
		if err := s.Copy(ctx, src, dest); err != nil {
			return err
		}
		return os.Remove(srcPath)
	}
	return nil
}
