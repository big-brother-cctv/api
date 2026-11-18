package database

import (
	"crypto/rand"
	"encoding/base64"
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
	if err := db.AutoMigrate(&models.User{}, &models.Camera{}, &models.MotionAlert{}, &models.PushSubscription{}, &models.AppSetting{}); err != nil {
		return nil, err
	}

	// Inicializador de datos (equivalente a DataInitializer.java)
	seedInitialUser(db, cfg)

	return db, nil
}

func seedInitialUser(db *gorm.DB, cfg *config.Config) {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		return
	}

	password := cfg.AdminInitialPassword
	generated := false
	if password == "" {
		var err error
		password, err = randomPassword(18)
		if err != nil {
			log.Printf("Error al generar password inicial: %v", err)
			return
		}
		generated = true
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error al hashear password inicial: %v", err)
		return
	}

	adminUser := models.User{
		Username:       "admin",
		Name:           "Administrator",
		Email:          "admin@bigbrother.local",
		HashedPassword: string(hashedPassword),
		Role:           "admin",
		Permissions:    "admin:all",
	}

	if err := db.Create(&adminUser).Error; err != nil {
		log.Printf("Error al crear usuario administrador inicial: %v", err)
		return
	}

	if generated {
		log.Printf("==========================================================")
		log.Printf(" Usuario inicial creado: admin")
		log.Printf(" Password generada (se muestra UNA sola vez): %s", password)
		log.Printf(" Cámbiala tras el primer login o define ADMIN_INITIAL_PASSWORD.")
		log.Printf("==========================================================")
	} else {
		log.Printf("Usuario inicial creado: admin (password de ADMIN_INITIAL_PASSWORD)")
	}
}

func randomPassword(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
