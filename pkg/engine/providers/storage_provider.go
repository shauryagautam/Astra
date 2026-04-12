package providers

import (
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/storage"
)

type StorageProvider struct {
	engine.BaseProvider
	disks map[string]storage.Storage
}

func NewStorageProvider() *StorageProvider {
	return &StorageProvider{
		disks: make(map[string]storage.Storage),
	}
}

func (p *StorageProvider) Name() string { return "storage" }

func (p *StorageProvider) Register(a *engine.App) error {
	rootDir := a.Env().String("STORAGE_LOCAL_ROOT", "storage")
	defaultDisk := storage.NewLocalStorage(rootDir)
	p.disks["local"] = defaultDisk

	slog.Info("✓ Storage service set", "default_disk", "local")
	return nil
}

