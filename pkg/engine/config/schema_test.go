package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAllowsMinimalHTTPOnlyConfig(t *testing.T) {
	cfg := &AstraConfig{
		App: AppConfig{
			Key: "01234567890123456789012345678901",
		},
		Database: DatabaseConfig{
			URL: "postgres://localhost:5432/astra",
		},
	}

	require.NoError(t, cfg.Validate())
}

func TestValidateChecksConfiguredSecrets(t *testing.T) {
	cfg := &AstraConfig{
		App: AppConfig{
			Key: "short",
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "APP_KEY")
}

func TestValidateRequiredUsesConfigValues(t *testing.T) {
	cfg := &AstraConfig{
		App: AppConfig{
			Name: "astra",
		},
		Storage: StorageConfig{
			S3Bucket: "framework-assets",
		},
	}

	require.NoError(t, cfg.ValidateRequired("APP_NAME", "S3_BUCKET"))
	require.ErrorContains(t, cfg.ValidateRequired("RESEND_API_KEY"), "RESEND_API_KEY")
}
