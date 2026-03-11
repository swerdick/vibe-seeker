package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateToken_RoundTrip(t *testing.T) {
	secret := "test-secret"
	tokenString, err := CreateToken(secret, "spotify123", "Test User")
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	claims, err := ParseToken(secret, tokenString)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.SpotifyID != "spotify123" {
		t.Errorf("SpotifyID = %q, want %q", claims.SpotifyID, "spotify123")
	}

	if claims.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", claims.DisplayName, "Test User")
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	tokenString, err := CreateToken("secret-one", "spotify123", "Test User")
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	_, err = ParseToken("secret-two", tokenString)
	if err == nil {
		t.Fatal("expected error when parsing with wrong secret")
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	secret := "test-secret"

	claims := Claims{
		SpotifyID:   "spotify123",
		DisplayName: "Test User",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = ParseToken(secret, tokenString)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParseToken_InvalidString(t *testing.T) {
	_, err := ParseToken("secret", "not-a-valid-jwt")
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}
