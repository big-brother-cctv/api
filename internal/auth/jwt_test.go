package auth

import (
	"testing"
)

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := "8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n"
	svc := NewJWTService(secret)

	username := "admin"
	token, err := svc.GenerateToken(username)
	if err != nil {
		t.Fatalf("Error generating token: %v", err)
	}

	if token == "" {
		t.Fatal("Expected non-empty token")
	}

	extractedUser, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("Error validating token: %v", err)
	}

	if extractedUser != username {
		t.Fatalf("Expected %s, got %s", username, extractedUser)
	}
}

func TestInvalidTokenValidation(t *testing.T) {
	secret := "8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n"
	svc := NewJWTService(secret)

	_, err := svc.ValidateToken("invalid.token.here")
	if err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}
}
