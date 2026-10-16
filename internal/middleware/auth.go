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
			c.Set("role", "admin")
			c.Set("is_internal", true)
			c.Next()
			return
		}

		// 2. Comprobación de token JWT de usuario
		username, role, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("username", username)
		c.Set("role", role)
		c.Set("is_internal", false)
		c.Next()
	}
}

// RequireRole valida que el usuario autenticado tenga uno de los roles permitidos
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied: missing role clearance"})
			return
		}

		userRole, _ := roleVal.(string)
		for _, allowed := range allowedRoles {
			if strings.EqualFold(userRole, allowed) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "Acceso denegado: permisos insuficientes para realizar esta acción",
		})
	}
}
