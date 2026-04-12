package platform

import (
	"context"
	"fmt"
	"os"
)

// SecretProvider defines the interface for fetching sensitive configuration values.
type SecretProvider interface {
	GetSecret(ctx context.Context, key string) (string, error)
}

// EnvSecretProvider fetches secrets directly from the environment.
type EnvSecretProvider struct{}

func NewEnvSecretProvider() *EnvSecretProvider {
	return &EnvSecretProvider{}
}

func (p *EnvSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	if val, ok := os.LookupEnv(key); ok {
		return val, nil
	}
	return "", fmt.Errorf("secret %q not found in environment", key)
}

// EncryptedSecretProvider fetches secrets from local encrypted stores (.env.vault).
type EncryptedSecretProvider struct {
	// Stub for reading encrypted .env.vault files using Astra's encryption utils
}

func NewEncryptedSecretProvider() *EncryptedSecretProvider {
	return &EncryptedSecretProvider{}
}

func (p *EncryptedSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Stub: In a real implementation, this would decrypt local vault files.
	return "", fmt.Errorf("secret %q not found in encrypted file", key)
}

// CloudSecretProvider is a stub for cloud-native secret managers (AWS/GCP/Vault).
type CloudSecretProvider struct{}

func NewCloudSecretProvider() *CloudSecretProvider {
	return &CloudSecretProvider{}
}

func (p *CloudSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Stub: In a real implementation, this might call AWS Secrets Manager.
	return "", fmt.Errorf("secret %q not found in cloud provider", key)
}

// ChainSecretProvider implements the Chain of Responsibility pattern for secrets.
type ChainSecretProvider struct {
	providers []SecretProvider
}

// NewChainSecretProvider creates a new chain of secret providers.
func NewChainSecretProvider(providers ...SecretProvider) *ChainSecretProvider {
	return &ChainSecretProvider{providers: providers}
}

// GetSecret iterates through the provider chain, returning the first successful match.
func (p *ChainSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	for _, provider := range p.providers {
		val, err := provider.GetSecret(ctx, key)
		if err == nil && val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("secret %q not found in any provider", key)
}
