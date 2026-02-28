package contracts

// HashDriverContract defines the interface for a hashing driver (Argon2, Bcrypt).
// Mirrors AdonisJS's HashDriverContract.
type HashDriverContract interface {
	// Make hashes the given value.
	// Mirrors: await Hash.make(value)
	Make(value string) (string, error)

	// Verify checks if a plain value matches a hash.
	// Mirrors: await Hash.verify(hash, value)
	Verify(hashedValue string, plainValue string) (bool, error)

	// NeedsRehash checks if the hash needs to be rehashed (e.g., config changed).
	NeedsRehash(hashedValue string) bool
}

// HashContract defines the Hash module interface.
// Provides a unified API that delegates to the configured driver.
// Mirrors AdonisJS's Hash module.
type HashContract interface {
	HashDriverContract

	// Use switches to a specific hash driver by name.
	// Mirrors: Hash.use('argon2')
	Use(driver string) HashDriverContract
}
