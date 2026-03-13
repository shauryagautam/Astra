package storage

import (
	"context"
	"log/slog"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/orm"
)

// Provider implements core.Provider for the storage package.
type Provider struct {
	core.BaseProvider
	disks map[string]Storage
}

// NewProvider creates a new storage provider.
func NewProvider() *Provider {
	return &Provider{
		disks: make(map[string]Storage),
	}
}

// Register registers the storage service in the container.
func (p *Provider) Register(a *core.App) error {
	// Default to local storage for now
	rootDir := a.Env.String("STORAGE_LOCAL_ROOT", "storage")
	defaultDisk := NewLocalStorage(rootDir)
	p.disks["local"] = defaultDisk

	a.Register("storage", defaultDisk)
	slog.Info("✓ Storage service registered", "default_disk", "local")
	return nil
}

// Boot wires the storage service into other components (e.g., ORM).
func (p *Provider) Boot(a *core.App) error {
	orm.SetAttachmentResolver(func(disk, path string) (string, error) {
		d, ok := p.disks[disk]
		if !ok {
			d = p.disks["local"]
		}
		return d.URL(path)
	})
	return nil
}

// Shutdown is a no-op for storage.
func (p *Provider) Shutdown(ctx context.Context, a *core.App) error {
	return nil
}
