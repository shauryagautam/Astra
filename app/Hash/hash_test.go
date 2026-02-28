package hash

import (
	"strings"
	"testing"
)

func TestArgon2MakeAndVerify(t *testing.T) {
	driver := NewArgon2Driver(DefaultArgon2Config())

	hashed, err := driver.Make("my-secret-password")
	if err != nil {
		t.Fatalf("Make failed: %v", err)
	}

	if !strings.HasPrefix(hashed, "$argon2id$") {
		t.Fatalf("expected argon2id prefix, got: %s", hashed[:20])
	}

	match, err := driver.Verify(hashed, "my-secret-password")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !match {
		t.Fatal("Verify should return true for correct password")
	}

	noMatch, err := driver.Verify(hashed, "wrong-password")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if noMatch {
		t.Fatal("Verify should return false for wrong password")
	}
}

func TestBcryptMakeAndVerify(t *testing.T) {
	driver := NewBcryptDriver(DefaultBcryptConfig())

	hashed, err := driver.Make("bcrypt-password")
	if err != nil {
		t.Fatalf("Make failed: %v", err)
	}

	if !strings.HasPrefix(hashed, "$2a$") {
		t.Fatalf("expected bcrypt prefix, got: %s", hashed[:4])
	}

	match, err := driver.Verify(hashed, "bcrypt-password")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !match {
		t.Fatal("Verify should return true for correct password")
	}

	noMatch, err := driver.Verify(hashed, "wrong")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if noMatch {
		t.Fatal("Verify should return false for wrong password")
	}
}

func TestManagerAutoDetect(t *testing.T) {
	mgr := NewManager("argon2", DefaultArgon2Config(), DefaultBcryptConfig())

	// Hash with argon2
	argonHash, _ := mgr.Use("argon2").Make("test123")
	match, _ := mgr.Verify(argonHash, "test123")
	if !match {
		t.Fatal("Manager should auto-detect argon2 and verify correctly")
	}

	// Hash with bcrypt
	bcryptHash, _ := mgr.Use("bcrypt").Make("test456")
	match, _ = mgr.Verify(bcryptHash, "test456")
	if !match {
		t.Fatal("Manager should auto-detect bcrypt and verify correctly")
	}
}

func TestNeedsRehash(t *testing.T) {
	driver := NewArgon2Driver(DefaultArgon2Config())
	hashed, _ := driver.Make("password")

	if driver.NeedsRehash(hashed) {
		t.Fatal("should not need rehash with same config")
	}

	// Change config
	differentDriver := NewArgon2Driver(Argon2Config{
		Memory:      131072,
		Iterations:  4,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	})

	if !differentDriver.NeedsRehash(hashed) {
		t.Fatal("should need rehash with different config")
	}
}
