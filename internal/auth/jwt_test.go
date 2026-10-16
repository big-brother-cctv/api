package auth

import (
	"testing"
)

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := "8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n"
	svc := NewJWTService(secret)

	username := "admin"
	token, err := svc.GenerateToken(username, "admin")
	if err != nil {
		t.Fatalf("Error generating token: %v", err)
	}

	if token == "" {
		t.Fatal("Expected non-empty token")
	}

	extractedUser, role, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("Error validating token: %v", err)
	}

	if extractedUser != username {
		t.Fatalf("Expected %s, got %s", username, extractedUser)
	}
	if role != "admin" {
		t.Fatalf("Expected role admin, got %s", role)
	}
}

func TestInvalidTokenValidation(t *testing.T) {
	secret := "8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n"
	svc := NewJWTService(secret)

	_, _, err := svc.ValidateToken("invalid.token.here")
	if err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}
}
