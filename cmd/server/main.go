package main

import (
	"log"
	"net/http"

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

	// Configurar router Gin
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())

	// Health check (compatible con Spring Boot Actuator)
	r.GET("/actuator/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	api := r.Group("/api")
	{
		// Rutas públicas de autenticación
		authGroup := api.Group("/auth")
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
			// Cameras
			protected.GET("/cameras", cameraHandler.GetAll)
			protected.POST("/cameras", cameraHandler.Create)
			protected.GET("/cameras/search", cameraHandler.Search)
			protected.GET("/cameras/:id", cameraHandler.GetByID)
			protected.PUT("/cameras/:id", cameraHandler.Update)
			protected.DELETE("/cameras/:id", cameraHandler.Delete)

			// Users
			protected.GET("/users", userHandler.GetAll)
			protected.POST("/users", userHandler.Create)
			protected.GET("/users/me", userHandler.GetMe)
			protected.GET("/users/:id", userHandler.GetByID)
			protected.PUT("/users/:id", userHandler.Update)
			protected.DELETE("/users/:id", userHandler.Delete)

			// Uploads
			protected.POST("/upload/image", uploadHandler.UploadImage)
			protected.POST("/upload/video", uploadHandler.UploadVideo)
		}
	}

	addr := ":" + cfg.Port
	log.Printf("Big Brother Go Backend arrancando en %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Fallo al iniciar el servidor: %v", err)
	}
}
