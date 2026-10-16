package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware inyecta cabeceras de seguridad defensivas en todas las respuestas
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		c.Next()
	}
}

// ipTracker almacena el conteo de peticiones y tiempo para el rate limiting en memoria
type ipTracker struct {
	count     int
	resetTime time.Time
}

type RateLimiter struct {
	sync.Mutex
	limits   map[string]*ipTracker
	maxReqs  int
	window   time.Duration
}

func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limits:  make(map[string]*ipTracker),
		maxReqs: maxReqs,
		window:  window,
	}

	// Rutina de limpieza periódica para evitar fugas de memoria
	go func() {
		for {
			time.Sleep(window * 2)
			rl.Lock()
			now := time.Now()
			for ip, tracker := range rl.limits {
				if now.After(tracker.resetTime) {
					delete(rl.limits, ip)
				}
			}
			rl.Unlock()
		}
	}()

	return rl
}

// LimitMiddleware aplica rate limiting por dirección IP
func (rl *RateLimiter) LimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		rl.Lock()
		tracker, exists := rl.limits[ip]
		if !exists || now.After(tracker.resetTime) {
			rl.limits[ip] = &ipTracker{
				count:     1,
				resetTime: now.Add(rl.window),
			}
			rl.Unlock()
			c.Next()
			return
		}

		tracker.count++
		if tracker.count > rl.maxReqs {
			rl.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.ErrorResponse{
				Message: "Demasiadas peticiones. Por favor, espere un momento antes de reintentar.",
				Error:   "RateLimitExceeded",
			})
			return
		}

		rl.Unlock()
		c.Next()
	}
}
