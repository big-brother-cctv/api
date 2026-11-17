package config

import "testing"

func TestValidateRejectsInsecureSecretsInProduction(t *testing.T) {
	cases := []struct {
		name string
		jwt  string
		tok  string
	}{
		{"empty", "", ""},
		{"published jwt", "8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n", "a-very-long-random-internal-token-123"},
		{"published token", "a-very-long-random-jwt-secret-value-0123456789", "internal-token-pro"},
		{"short", "short", "short"},
	}
	for _, tc := range cases {
		cfg := &Config{AppEnv: "production", JWTSecret: tc.jwt, InternalToken: tc.tok}
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected error in production", tc.name)
		}
	}
}

func TestValidateAcceptsStrongSecrets(t *testing.T) {
	cfg := &Config{
		AppEnv:        "production",
		JWTSecret:     "a-very-long-random-jwt-secret-value-0123456789",
		InternalToken: "a-very-long-random-internal-token-123",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDevModeFillsDefaults(t *testing.T) {
	cfg := &Config{AppEnv: "dev"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("dev mode should not fail: %v", err)
	}
	if cfg.JWTSecret == "" || cfg.InternalToken == "" {
		t.Fatal("dev mode should fill missing secrets")
	}
}
