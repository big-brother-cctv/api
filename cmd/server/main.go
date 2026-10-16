package main

import (
	"log"
	"net/http"
	"time"

	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/database"
	"github.com/bigbrother/api/internal/handlers"
	"github.com/bigbrother/api/internal/mediamtx"
	"github.com/bigbrother/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	// Inicializar base de datos
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Error al conectar con la base de datos: %v", err)
	}

	// Inicializar servicios y handlers
	jwtService := auth.NewJWTService(cfg.JWTSecret)
	mtxService := mediamtx.NewService(cfg)
	mtxService.StartSyncLoop(db)

	authHandler := handlers.NewAuthHandler(db, jwtService)
	cameraHandler := handlers.NewCameraHandler(db, mtxService, cfg)
	userHandler := handlers.NewUserHandler(db)
	uploadHandler := handlers.NewUploadHandler(cfg)

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

	api := r.Group("/api")
	{
		// Rutas públicas de autenticación con rate limiting activo
		authGroup := api.Group("/auth")
		authGroup.Use(authRateLimiter.LimitMiddleware())
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/register", authHandler.Register)
		}

		// Heartbeat público/semiprotegido para agentes remotos (admite token opcional)
		api.POST("/cameras/heartbeat", cameraHandler.Heartbeat)

		// Rutas protegidas (JWT o Internal Token)
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(jwtService, cfg))
		{
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

			// Uploads: Solo operadores y administradores pueden subir snapshots y grabaciones
			uploadGroup := protected.Group("/upload")
			uploadGroup.Use(middleware.RequireRole("admin", "operator"))
			{
				uploadGroup.POST("/image", uploadHandler.UploadImage)
				uploadGroup.POST("/video", uploadHandler.UploadVideo)
			}
		}
	}

	addr := ":" + cfg.Port
	log.Printf("Big Brother Go Backend arrancando en %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Fallo al iniciar el servidor: %v", err)
	}
}
