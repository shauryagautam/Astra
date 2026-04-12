package http

import (
	"crypto/tls"
	"fmt"
	"os"
)

// TLSConfig holds TLS/HTTPS configuration
type TLSConfig struct {
	Enabled    bool
	CertFile   string
	KeyFile    string
	MinVersion uint16
}

// LoadTLSConfig loads TLS configuration from environment
func LoadTLSConfig() *TLSConfig {
	return &TLSConfig{
		Enabled:    os.Getenv("TLS_ENABLED") == "true",
		CertFile:   os.Getenv("TLS_CERT_FILE"),
		KeyFile:    os.Getenv("TLS_KEY_FILE"),
		MinVersion: tls.VersionTLS12,
	}
}

// GetTLSConfig returns a tls.Config with secure defaults
func (c *TLSConfig) GetTLSConfig() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	if c.CertFile == "" || c.KeyFile == "" {
		return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be set when TLS_ENABLED=true")
	}

	return &tls.Config{
		MinVersion:               c.MinVersion,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}
