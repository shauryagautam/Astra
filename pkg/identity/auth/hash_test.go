package auth_test

import (
	"testing"

	"github.com/shauryagautam/Astra/pkg/identity/auth"
	"github.com/stretchr/testify/assert"
)

func TestArgon2idHasher(t *testing.T) {
	hasher := auth.NewArgon2idHasher()
	hasher.Iterations = 1 // lower iterations for faster tests
	hasher.Memory = 16 * 1024

	password := "super_secret_password!"

	// Test Make
	hash, err := hasher.Make(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "$argon2id$v=19$")

	// Test Check
	assert.True(t, hasher.Check(password, hash))
	assert.False(t, hasher.Check("wrong_password", hash))

	// Test NeedsRehash
	assert.False(t, hasher.NeedsRehash(hash))

	// Alter parameters to force NeedsRehash = true
	hasher.Iterations = 2
	assert.True(t, hasher.NeedsRehash(hash))
}
