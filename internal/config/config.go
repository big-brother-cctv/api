package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port              string
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	JWTSecret         string
	InternalToken     string
	ContentManagerURL string
	MediaMTXAPIURL    string
	MediaMTXRTSPHost  string
	DataDir           string
	AlertsDir         string
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func LoadConfig() *Config {
	dataDir := getEnv("DATA_DIR", "./data")
	alertsDir := getEnv("ALERTS_DIR", dataDir+"/alerts")

	return &Config{
		Port:              getEnv("PORT", "8080"),
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            getEnv("DB_USER", "admin"),
		DBPassword:        getEnv("DB_PASSWORD", "admin"),
		DBName:            getEnv("DB_NAME", "bigbrother"),
		DBSSLMode:         getEnv("DB_SSLMODE", "disable"),
		JWTSecret:         getEnv("JWT_SECRET", "8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n"),
		InternalToken:     getEnv("INTERNAL_TOKEN", "internal-token-dev"),
		ContentManagerURL: getEnv("CONTENT_MANAGER_URL", "http://content-manager:8181"),
		MediaMTXAPIURL:    getEnv("MEDIAMTX_API_URL", "http://localhost:9997"),
		MediaMTXRTSPHost:  getEnv("MEDIAMTX_RTSP_HOST", "localhost"),
		DataDir:           dataDir,
		AlertsDir:         alertsDir,
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}
