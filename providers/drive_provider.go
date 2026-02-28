package providers

import (
	"path/filepath"

	drive "github.com/shaurya/adonis/app/Drive"
	"github.com/shaurya/adonis/contracts"
)

// DriveProvider registers the Drive manager and disks into the container.
// Mirrors AdonisJS's @adonisjs/drive provider.
type DriveProvider struct {
	BaseProvider
}

// NewDriveProvider creates a new DriveProvider.
func NewDriveProvider(app contracts.ApplicationContract) *DriveProvider {
	return &DriveProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds the Drive manager as a singleton.
func (p *DriveProvider) Register() error {
	p.App.Singleton("Adonis/Core/Drive", func(c contracts.ContainerContract) (any, error) {
		// Use the Env manager to get the default disk
		env, err := c.Make("Env")
		if err != nil {
			return nil, err
		}

		envManager := env.(*EnvManager)
		defaultDisk := envManager.Get("DRIVE_DISK", "local")

		manager := drive.NewDriveManager(defaultDisk)

		// Register the local disk
		rootPath := filepath.Join(p.App.AppRoot(), "storage")
		manager.RegisterDisk("local", drive.NewLocalDriver(rootPath))

		return manager, nil
	})

	p.App.Alias("Drive", "Adonis/Core/Drive")

	return nil
}

// Boot is a no-op for the DriveProvider.
func (p *DriveProvider) Boot() error {
	return nil
}
