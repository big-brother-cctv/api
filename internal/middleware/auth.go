package middleware

import (
	"net/http"
	"strings"

	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/config"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(jwtService *auth.JWTService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			return
		}

		tokenString := strings.TrimSpace(parts[1])

		// 1. Comprobación de token interno (usado por el agente de cámara en camera/src/modules/config.py)
		if cfg.InternalToken != "" && tokenString == cfg.InternalToken {
			c.Set("username", "internal_agent")
			c.Set("is_internal", true)
			c.Next()
			return
		}

		// 2. Comprobación de token JWT de usuario
		username, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("username", username)
		c.Set("is_internal", false)
		c.Next()
	}
}
