package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/config"
	"github.com/gin-gonic/gin"
)

// MediaCookieName es la cookie HttpOnly que autoriza lecturas de medios
// (SSE de alertas, snapshots y streams HLS/WebRTC vía nginx auth_request).
const MediaCookieName = "bb_media"

func bearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

// IsInternalToken compara en tiempo constante con el token interno de servicios.
func IsInternalToken(cfg *config.Config, token string) bool {
	return cfg.InternalToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(cfg.InternalToken)) == 1
}

// authenticateBearer valida un Bearer (token interno o JWT de sesión) y rellena el contexto.
func authenticateBearer(c *gin.Context, jwtService *auth.JWTService, cfg *config.Config, token string) bool {
	if IsInternalToken(cfg, token) {
		c.Set("username", "internal_agent")
		c.Set("role", "admin")
		c.Set("is_internal", true)
		return true
	}

	username, role, err := jwtService.ValidateToken(token)
	if err != nil {
		return false
	}
	c.Set("username", username)
	c.Set("role", role)
	c.Set("is_internal", false)
	return true
}

func AuthMiddleware(jwtService *auth.JWTService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		token, ok := bearerToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			return
		}

		if !authenticateBearer(c, jwtService, cfg, token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}
		c.Next()
	}
}

// MediaAuthMiddleware protege recursos que el navegador pide sin poder añadir
// cabeceras (EventSource, <img>, <video>, hls.js nativo en Safari): acepta
// Bearer como AuthMiddleware o, si no hay, la cookie de medios.
func MediaAuthMiddleware(jwtService *auth.JWTService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, ok := bearerToken(c); ok {
			if authenticateBearer(c, jwtService, cfg, token) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		cookie, err := c.Cookie(MediaCookieName)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Media session required"})
			return
		}
		username, role, err := jwtService.ValidateMediaToken(cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired media session"})
			return
		}
		c.Set("username", username)
		c.Set("role", role)
		c.Set("is_internal", false)
		c.Next()
	}
}

// SnapshotAuthMiddleware acepta una URL firmada (?exp=&sig=, para imágenes de notificaciones
// push) o, si no la hay, las credenciales de medios habituales.
func SnapshotAuthMiddleware(jwtService *auth.JWTService, cfg *config.Config) gin.HandlerFunc {
	media := MediaAuthMiddleware(jwtService, cfg)
	return func(c *gin.Context) {
		if sig := c.Query("sig"); sig != "" {
			if jwtService.VerifySnapshotSignature(c.Param("filepath"), c.Query("exp"), sig, time.Now()) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired signed URL"})
			return
		}
		media(c)
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
