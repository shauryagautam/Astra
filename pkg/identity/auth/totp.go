package auth

import (
	"bytes"
	"encoding/base64"
	"image/png"

	"github.com/pquerna/otp/totp"
)

// TOTPManager handles Two-Factor Authentication using TOTP.
type TOTPManager struct {
	Issuer string
}

// NewTOTPManager creates a new TOTP manager.
func NewTOTPManager(issuer string) *TOTPManager {
	return &TOTPManager{Issuer: issuer}
}

// Generate creates a new TOTP secret and returns the key and a QR code as a base64 string.
func (m *TOTPManager) Generate(accountName string) (secret string, qrCodeBase64 string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      m.Issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", err
	}

	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		return "", "", err
	}

	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}

	qrCodeBase64 = base64.StdEncoding.EncodeToString(buf.Bytes())
	return key.Secret(), qrCodeBase64, nil
}

// Verify validates a TOTP code against a secret.
func (m *TOTPManager) Verify(code string, secret string) bool {
	return totp.Validate(code, secret)
}
