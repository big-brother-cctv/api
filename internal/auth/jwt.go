package auth

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secretKey []byte
}

func NewJWTService(secret string) *JWTService {
	keyBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil || len(keyBytes) == 0 {
		keyBytes = []byte(secret)
	}
	return &JWTService{secretKey: keyBytes}
}

func (s *JWTService) GenerateToken(username, role string) (string, error) {
	if role == "" {
		role = "operator"
	}
	claims := jwt.MapClaims{
		"sub":  username,
		"role": role,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(10 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *JWTService) ValidateToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid claims")
	}

	username, ok := claims["sub"].(string)
	if !ok || username == "" {
		return "", "", fmt.Errorf("subject missing from token")
	}

	role, _ := claims["role"].(string)
	if role == "" {
		role = "operator"
	}

	return username, role, nil
}
