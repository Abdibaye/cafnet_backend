package auth_test

import (
	"strings"
	"testing"

	"github.com/Abdibaye/cafnet_backend/internal/auth"
)

func TestPasswordHasher(t *testing.T) {
	// Use custom hasher with low memory/time for fast unit testing
	hasher := auth.NewCustomArgon2idHasher(1, 16*1024, 2, 32, 16)

	password := "SecretP@ssw0rd123"

	// 1. Hash password
	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$v=") {
		t.Fatalf("unexpected hash prefix format: %s", encoded)
	}

	// 2. Plaintext password is never in the hash string
	if strings.Contains(encoded, password) {
		t.Fatalf("plaintext password leaked in hash string")
	}

	// 3. Verify correct password
	match, err := hasher.Verify(encoded, password)
	if err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if !match {
		t.Fatalf("expected password verification to succeed, got false")
	}

	// 4. Verify wrong password
	matchWrong, err := hasher.Verify(encoded, "WrongPassword!")
	if err != nil {
		t.Fatalf("verify wrong password returned error: %v", err)
	}
	if matchWrong {
		t.Fatalf("expected wrong password verification to fail, got true")
	}

	// 5. Verify malformed hash
	_, err = hasher.Verify("invalid-hash-string", password)
	if err == nil {
		t.Fatalf("expected error for malformed hash, got nil")
	}
}
