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

func TestMediaTokenScopeSeparation(t *testing.T) {
	svc := NewJWTService("a-very-long-random-jwt-secret-value-0123456789")

	media, err := svc.GenerateMediaToken("viewer1", "viewer")
	if err != nil {
		t.Fatalf("Error generating media token: %v", err)
	}
	if _, _, err := svc.ValidateToken(media); err == nil {
		t.Fatal("media token must NOT be accepted as session token")
	}
	user, role, err := svc.ValidateMediaToken(media)
	if err != nil || user != "viewer1" || role != "viewer" {
		t.Fatalf("media token should validate as media: user=%q role=%q err=%v", user, role, err)
	}

	session, _ := svc.GenerateToken("viewer1", "viewer")
	if _, _, err := svc.ValidateMediaToken(session); err == nil {
		t.Fatal("session token must NOT be accepted as media token")
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
