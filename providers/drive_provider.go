package providers

import (
	"context"
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

		// Register S3 disk if configured
		s3Region := envManager.Get("S3_REGION", "")
		s3Bucket := envManager.Get("S3_BUCKET", "")
		if s3Region != "" && s3Bucket != "" {
			s3Cfg := drive.S3Config{
				Region: s3Region,
				Bucket: s3Bucket,
			}
			// We use background context for initialization
			s3Driver, err := drive.NewS3Driver(context.Background(), s3Cfg)
			if err == nil {
				manager.RegisterDisk("s3", s3Driver)
			}
		}

		return manager, nil
	})

	p.App.Alias("Drive", "Adonis/Core/Drive")

	return nil
}

// Boot is a no-op for the DriveProvider.
func (p *DriveProvider) Boot() error {
	return nil
}
