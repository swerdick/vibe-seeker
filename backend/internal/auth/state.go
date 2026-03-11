package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateState produces a cryptographically random hex string for OAuth CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
