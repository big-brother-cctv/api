package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/bigbrother/api/internal/alerts"
	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/database"
	"github.com/bigbrother/api/internal/handlers"
	"github.com/bigbrother/api/internal/mediamtx"
	"github.com/bigbrother/api/internal/middleware"
	"github.com/bigbrother/api/internal/retention"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[SECURITY] %v", err)
	}

	// Inicializar base de datos
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Error al conectar con la base de datos: %v", err)
	}

	jwtService := auth.NewJWTService(cfg.JWTSecret)
	mtxService := mediamtx.NewService(cfg)
	mtxService.StartSyncLoop(db)

	// Retención de alertas: el PVC de snapshots se comparte con otros servicios del homelab
	retention.New(retention.GormStore{DB: db}, cfg.AlertsDir, retention.Policy{
		MaxAge:   time.Duration(cfg.AlertRetentionDays) * 24 * time.Hour,
		MaxBytes: int64(cfg.AlertMaxStorageMB) * 1024 * 1024,
	}).Start(context.Background(), time.Hour)

	r := newRouter(cfg, db, jwtService, mtxService, alerts.NewHub())

	addr := ":" + cfg.Port
	log.Printf("Big Brother Go Backend arrancando en %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Fallo al iniciar el servidor: %v", err)
	}
}

// newRouter construye el router HTTP. Separado de main() para poder testear el enrutado.
func newRouter(cfg *config.Config, db *gorm.DB, jwtService *auth.JWTService, mtxService *mediamtx.Service, alertsHub *alerts.Hub) *gin.Engine {
	alertHandler := handlers.NewAlertHandler(db, alertsHub, cfg)
	authHandler := handlers.NewAuthHandler(db, jwtService)
	cameraHandler := handlers.NewCameraHandler(db, mtxService, cfg)
	userHandler := handlers.NewUserHandler(db)
	uploadHandler := handlers.NewUploadHandler(cfg)
	mtxAuthHandler := handlers.NewMediaMTXAuthHandler(cfg)

	// Rate limiter para endpoints sensibles de autenticación (máx 20 intentos por minuto por IP)
	authRateLimiter := middleware.NewRateLimiter(20, time.Minute)

	// Configurar router Gin
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())

	// Health check (compatible con Spring Boot Actuator)
	r.GET("/actuator/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	// Credenciales de medios: Bearer o cookie HttpOnly bb_media (EventSource, <img>, <video>)
	mediaAuth := middleware.MediaAuthMiddleware(jwtService, cfg)

	// Capturas de movimiento: solo usuarios autenticados
	snapshots := r.Group("/api/alerts/snapshots")
	snapshots.Use(mediaAuth)
	snapshots.Static("/", alertHandler.StorageDir())

	// Endpoint authHTTPAddress de MediaMTX (authMethod: http). Fuera de /api para que el
	// nginx público de Telescreen no lo exponga; solo lo llama MediaMTX dentro del clúster.
	r.POST("/internal/mediamtx/auth", mtxAuthHandler.Auth)

	api := r.Group("/api")
	{
		// Rutas públicas de autenticación con rate limiting activo
		authGroup := api.Group("/auth")
		authGroup.Use(authRateLimiter.LimitMiddleware())
		{
			authGroup.POST("/login", authHandler.Login)
			// Auto-registro desactivado por defecto; si se activa, siempre crea usuarios "viewer"
			if cfg.AllowRegistration {
				authGroup.POST("/register", authHandler.Register)
			}
		}

		// Ingesta de alertas de movimiento (solo Internal Token de servicios)
		api.POST("/alerts", alertHandler.Create)

		// Cierre de sesión de medios: público para poder limpiar la cookie aunque el JWT haya caducado
		api.DELETE("/media/session", authHandler.EndMediaSession)

		// Recursos de medios que el navegador pide sin cabeceras (cookie bb_media)
		media := api.Group("")
		media.Use(mediaAuth)
		{
			// Flujo SSE en tiempo real para Telescreen (Server-Sent Events)
			media.GET("/alerts/stream", alertHandler.Stream)
			// Verificación para nginx auth_request (/stream/ HLS y /whep/ WebRTC)
			media.GET("/media/check", authHandler.MediaCheck)
		}

		// Rutas protegidas (JWT o Internal Token)
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(jwtService, cfg))
		{
			// Sesión de medios (cookie HttpOnly) a partir del Bearer actual
			protected.POST("/media/session", authHandler.StartMediaSession)
			// Credenciales de publicación RTSP/RTMP para móviles y OBS
			protected.GET("/media/publish-credentials", middleware.RequireRole("admin", "operator"), mtxAuthHandler.PublishCredentials)

			// Agentes de cámara: heartbeat y auto-registro (Internal Token u operador)
			protected.POST("/cameras/heartbeat", middleware.RequireRole("admin", "operator"), cameraHandler.Heartbeat)

			// Configuración unificada de Motion (motion-detector con Internal Token)
			protected.GET("/motion/config", middleware.RequireRole("admin"), cameraHandler.GetMotionConfig)

			// Cameras: Lectura para todos los roles válidos, Modificación para operator/admin, Eliminación para admin
			protected.GET("/cameras", cameraHandler.GetAll)
			protected.GET("/cameras/search", cameraHandler.Search)
			protected.GET("/cameras/:id", cameraHandler.GetByID)
			protected.POST("/cameras", middleware.RequireRole("admin", "operator"), cameraHandler.Create)
			protected.PUT("/cameras/:id", middleware.RequireRole("admin", "operator"), cameraHandler.Update)
			protected.DELETE("/cameras/:id", middleware.RequireRole("admin"), cameraHandler.Delete)

			// Users: Listado restringido a operator/admin; Creación y borrado exclusivo para admin; Modificación asegurada
			protected.GET("/users", middleware.RequireRole("admin", "operator"), userHandler.GetAll)
			protected.POST("/users", middleware.RequireRole("admin"), userHandler.Create)
			protected.GET("/users/me", userHandler.GetMe)
			protected.GET("/users/:id", userHandler.GetByID)
			protected.PUT("/users/:id", userHandler.Update)
			protected.DELETE("/users/:id", middleware.RequireRole("admin"), userHandler.Delete)

			// Alerts: Consulta, acuses de recibo y gestión de alertas de movimiento
			protected.GET("/alerts", alertHandler.GetAll)
			protected.GET("/alerts/unread-count", alertHandler.GetUnreadCount)
			protected.POST("/alerts/:id/ack", alertHandler.Acknowledge)
			protected.PUT("/alerts/:id/ack", alertHandler.Acknowledge)
			protected.POST("/alerts/ack-all", alertHandler.AcknowledgeAll)
			protected.PUT("/alerts/ack-all", alertHandler.AcknowledgeAll)
			protected.DELETE("/alerts/:id", middleware.RequireRole("admin", "operator"), alertHandler.Delete)
			protected.DELETE("/alerts", middleware.RequireRole("admin"), alertHandler.ClearAll)

			// Uploads: Solo operadores y administradores pueden subir snapshots y grabaciones
			uploadGroup := protected.Group("/upload")
			uploadGroup.Use(middleware.RequireRole("admin", "operator"))
			{
				uploadGroup.POST("/image", uploadHandler.UploadImage)
				uploadGroup.POST("/video", uploadHandler.UploadVideo)
			}
		}
	}

	return r
}
