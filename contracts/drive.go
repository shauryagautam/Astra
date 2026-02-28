package contracts

import (
	"io"
)

// DriveContract defines the file storage interface.
// Mirrors AdonisJS's @adonisjs/drive module.
type DriveContract interface {
	// Disk returns a named disk instance.
	Disk(name string) DiskContract

	// Put saves a file to the default disk.
	Put(path string, contents []byte) error

	// PutStream saves a file stream to the default disk.
	PutStream(path string, reader io.Reader) error

	// Get retrieves file contents.
	Get(path string) ([]byte, error)

	// Exists checks if a file exists.
	Exists(path string) (bool, error)

	// Delete removes a file.
	Delete(path string) error

	// Url returns a public URL for a file.
	Url(path string) string
}

// DiskContract defines operations on a specific storage disk.
type DiskContract interface {
	// Put saves a file.
	Put(path string, contents []byte) error

	// PutStream saves a file stream.
	PutStream(path string, reader io.Reader) error

	// Get retrieves file contents.
	Get(path string) ([]byte, error)

	// Exists checks if a file exists.
	Exists(path string) (bool, error)

	// Delete removes a file.
	Delete(path string) error

	// Url returns a public URL for a file.
	Url(path string) string
}
