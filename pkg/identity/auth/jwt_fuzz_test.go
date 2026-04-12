package auth

import (
	"strings"
	"testing"
)

// FuzzJWTVerify verifies that no token string causes a panic in Verify.
// Run with: go test -fuzz=FuzzJWTVerify ./auth/ -fuzztime=30s
func FuzzJWTVerify(f *testing.F) {
	// Seed corpus with valid and degenerate JWT shapes
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("a.b.c")
	f.Add("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ")
	f.Add(strings.Repeat("A", 8192)) // oversized
	f.Add("eyJhbGciOiJub25lIn0.e30.")  // alg:none attack
	f.Add("null")

	mgr := &JWTManager{
		keys:        map[string][]byte{"default": []byte("supersecretkey-at-least-32-bytes!!")},
		activeKeyID: "default",
	}

	f.Fuzz(func(t *testing.T, token string) {
		// Must not panic — error is expected for invalid tokens
		_, _ = mgr.Verify(token)
	})
}

// FuzzJWTLoadSecrets verifies that no secret string causes a panic during key loading.
func FuzzJWTLoadSecrets(f *testing.F) {
	f.Add("")
	f.Add("secret")
	f.Add("kid1:secret1,kid2:secret2")
	f.Add(strings.Repeat("k:v,", 1000))
	f.Add(":::")
	f.Add("k:" + strings.Repeat("x", 4096))

	f.Fuzz(func(t *testing.T, secretStr string) {
		mgr := &JWTManager{
			keys: make(map[string][]byte),
		}
		// Must not panic
		mgr.loadSecrets(secretStr)
	})
}
