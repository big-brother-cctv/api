package database

import (
	"log"

	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	// Migraciones automáticas manteniendo nombres de tabla users, cameras y alerts
	if err := db.AutoMigrate(&models.User{}, &models.Camera{}, &models.MotionAlert{}); err != nil {
		return nil, err
	}

	// Inicializador de datos (equivalente a DataInitializer.java)
	seedInitialUser(db)

	return db, nil
}

func seedInitialUser(db *gorm.DB) {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error al hashear password inicial: %v", err)
			return
		}

		adminUser := models.User{
			Username:       "admin",
			Name:           "Raul Del Valle",
			Email:          "admin@bandanize.com",
			HashedPassword: string(hashedPassword),
			Role:           "admin",
			Permissions:    "admin:all",
			Disabled:       false,
			Photo:          "",
		}

		if err := db.Create(&adminUser).Error; err != nil {
			log.Printf("Error al crear usuario administrador inicial: %v", err)
		} else {
			log.Printf("Initial user created: %s", adminUser.Username)
		}
	} else {
		// Asegurar que el usuario admin existente tenga rol de admin y permisos
		db.Model(&models.User{}).Where("username = ? AND (role = '' OR role IS NULL)", "admin").
			Updates(map[string]interface{}{
				"role":        "admin",
				"permissions": "admin:all",
			})
	}
}
