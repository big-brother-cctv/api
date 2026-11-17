package auth

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// SessionTTL es la vida de los tokens de sesión (Bearer) del dashboard.
	SessionTTL = 10 * time.Hour
	// MediaTTL es la vida de la cookie de medios (SSE, snapshots, HLS/WebRTC).
	// Telescreen la renueva periódicamente mientras la sesión siga activa.
	MediaTTL = 2 * time.Hour

	scopeClaim = "scope"
	scopeMedia = "media"
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

func (s *JWTService) sign(claims jwt.MapClaims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secretKey)
}

// GenerateToken emite el token de sesión usado como Bearer en la API REST.
func (s *JWTService) GenerateToken(username, role string) (string, error) {
	if role == "" {
		role = "operator"
	}
	now := time.Now()
	return s.sign(jwt.MapClaims{
		"sub":  username,
		"role": role,
		"iat":  now.Unix(),
		"exp":  now.Add(SessionTTL).Unix(),
	})
}

// GenerateMediaToken emite un token de alcance reducido que solo sirve para leer
// medios (SSE de alertas, snapshots y streams). Se entrega como cookie HttpOnly.
func (s *JWTService) GenerateMediaToken(username, role string) (string, error) {
	now := time.Now()
	return s.sign(jwt.MapClaims{
		"sub":      username,
		"role":     role,
		scopeClaim: scopeMedia,
		"iat":      now.Unix(),
		"exp":      now.Add(MediaTTL).Unix(),
	})
}

// ValidateToken valida un token de sesión. Rechaza tokens con scope (p. ej. media),
// para que la cookie de medios no pueda usarse como credencial de la API.
func (s *JWTService) ValidateToken(tokenString string) (string, string, error) {
	claims, err := s.parse(tokenString)
	if err != nil {
		return "", "", err
	}
	if scope, _ := claims[scopeClaim].(string); scope != "" {
		return "", "", fmt.Errorf("scoped token not valid as session token")
	}
	return extractIdentity(claims)
}

// ValidateMediaToken valida un token de medios (cookie).
func (s *JWTService) ValidateMediaToken(tokenString string) (string, string, error) {
	claims, err := s.parse(tokenString)
	if err != nil {
		return "", "", err
	}
	if scope, _ := claims[scopeClaim].(string); scope != scopeMedia {
		return "", "", fmt.Errorf("token is not a media token")
	}
	return extractIdentity(claims)
}

func (s *JWTService) parse(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	// jwt v5.0 solo valida exp si existe: exigimos que siempre esté presente.
	if _, err := claims.GetExpirationTime(); err != nil || claims["exp"] == nil {
		return nil, fmt.Errorf("token without expiration")
	}
	return claims, nil
}

func extractIdentity(claims jwt.MapClaims) (string, string, error) {
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
