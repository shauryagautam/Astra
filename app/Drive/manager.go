package drive

import (
	"fmt"
	"io"
	"sync"

	"github.com/shaurya/astra/contracts"
)

// DriveManager manages multiple storage disks.
type DriveManager struct {
	mu          sync.RWMutex
	disks       map[string]contracts.DiskContract
	defaultDisk string
}

// NewDriveManager creates a new DriveManager.
func NewDriveManager(defaultDisk string) *DriveManager {
	return &DriveManager{
		disks:       make(map[string]contracts.DiskContract),
		defaultDisk: defaultDisk,
	}
}

// RegisterDisk registers a named disk instance.
func (m *DriveManager) RegisterDisk(name string, disk contracts.DiskContract) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disks[name] = disk
}

// Disk returns a named disk instance.
func (m *DriveManager) Disk(name string) contracts.DiskContract {
	m.mu.RLock()
	defer m.mu.RUnlock()
	disk, ok := m.disks[name]
	if !ok {
		panic(fmt.Sprintf("Drive disk '%s' not registered", name))
	}
	return disk
}

// Default returns the default disk.
func (m *DriveManager) Default() contracts.DiskContract {
	return m.Disk(m.defaultDisk)
}

// Put forwarder for the default disk.
func (m *DriveManager) Put(path string, contents []byte) error {
	return m.Default().Put(path, contents)
}

// PutStream forwarder for the default disk.
func (m *DriveManager) PutStream(path string, reader io.Reader) error {
	return m.Default().PutStream(path, reader)
}

// Get forwarder for the default disk.
func (m *DriveManager) Get(path string) ([]byte, error) {
	return m.Default().Get(path)
}

// Exists forwarder for the default disk.
func (m *DriveManager) Exists(path string) (bool, error) {
	return m.Default().Exists(path)
}

// Delete forwarder for the default disk.
func (m *DriveManager) Delete(path string) error {
	return m.Default().Delete(path)
}

// Url forwarder for the default disk.
func (m *DriveManager) Url(path string) string {
	return m.Default().Url(path)
}

// Ensure DriveManager implements DriveContract.
var _ contracts.DriveContract = (*DriveManager)(nil)
