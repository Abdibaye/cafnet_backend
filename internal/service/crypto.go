package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
)

// GenerateSecureRandomBytes reads n cryptographically secure random bytes from crypto/rand.
func GenerateSecureRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}
	return b, nil
}

// GenerateSecureSessionToken generates a 32-byte (256-bit entropy) hex-encoded opaque token.
func GenerateSecureSessionToken() (string, error) {
	b, err := GenerateSecureRandomBytes(32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateRandomOTP generates a cryptographically random 6-digit numeric string (100000 - 999999).
func GenerateRandomOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("failed to generate random OTP: %w", err)
	}
	return strconv.FormatInt(n.Int64()+100000, 10), nil
}

// HashToken computes a deterministic SHA-256 hex digest of an opaque session credential.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashOTP computes a salted SHA-256 hex digest of an OTP verification code.
func HashOTP(phone, otp string) string {
	data := fmt.Sprintf("%s:%s", phone, otp)
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
