package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv               string
	Port                 string
	DBHost               string
	DBPort               string
	DBUser               string
	DBPassword           string
	DBName               string
	DBSSLMode            string
	JWTSecret            string
	InternalToken        string
	MediaPublishToken    string
	MediaReadToken       string
	AllowRegistration    bool
	AdminInitialPassword string
	ContentManagerURL    string
	MediaMTXAPIURL       string
	MediaMTXRTSPHost     string
	DataDir              string
	AlertsDir            string
	AlertRetentionDays   int
	AlertMaxStorageMB    int
}

// Valores que estuvieron publicados en el repositorio. Nunca deben aceptarse fuera de desarrollo.
var knownInsecureSecrets = map[string]bool{
	"8Q4n5v1Qw8v9wF3n9z7k4p1b2c3d4e5f6g7h8i9j0k1l2m3n": true,
	"internal-token-dev": true,
	"internal-token-pro": true,
	"changeme":           true,
	"secret":             true,
}

const (
	minJWTSecretLen     = 32
	minInternalTokenLen = 24
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt lee un entero >= 0; si falta o es inválido usa el valor por defecto.
func getEnvInt(key string, defaultVal int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		log.Printf("[CONFIG] %s=%q no es un entero >= 0; usando %d", key, v, defaultVal)
		return defaultVal
	}
	return n
}

func LoadConfig() *Config {
	dataDir := getEnv("DATA_DIR", "./data")
	alertsDir := getEnv("ALERTS_DIR", dataDir+"/alerts")

	return &Config{
		AppEnv:               strings.ToLower(getEnv("APP_ENV", "production")),
		Port:                 getEnv("PORT", "8080"),
		DBHost:               getEnv("DB_HOST", "localhost"),
		DBPort:               getEnv("DB_PORT", "5432"),
		DBUser:               getEnv("DB_USER", "admin"),
		DBPassword:           getEnv("DB_PASSWORD", "admin"),
		DBName:               getEnv("DB_NAME", "bigbrother"),
		DBSSLMode:            getEnv("DB_SSLMODE", "disable"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		InternalToken:        os.Getenv("INTERNAL_TOKEN"),
		MediaPublishToken:    os.Getenv("MEDIA_PUBLISH_TOKEN"),
		MediaReadToken:       os.Getenv("MEDIA_READ_TOKEN"),
		AllowRegistration:    strings.EqualFold(os.Getenv("ALLOW_REGISTRATION"), "true"),
		AdminInitialPassword: os.Getenv("ADMIN_INITIAL_PASSWORD"),
		ContentManagerURL:    getEnv("CONTENT_MANAGER_URL", "http://content-manager:8181"),
		MediaMTXAPIURL:       getEnv("MEDIAMTX_API_URL", "http://localhost:9997"),
		MediaMTXRTSPHost:     getEnv("MEDIAMTX_RTSP_HOST", "localhost"),
		DataDir:              dataDir,
		AlertsDir:            alertsDir,
		AlertRetentionDays:   getEnvInt("ALERT_RETENTION_DAYS", 30),
		AlertMaxStorageMB:    getEnvInt("ALERT_MAX_STORAGE_MB", 2048),
	}
}

// IsDev indica si la API corre en modo desarrollo (APP_ENV=dev|development|local).
func (c *Config) IsDev() bool {
	switch c.AppEnv {
	case "dev", "development", "local":
		return true
	}
	return false
}

// Validate comprueba que los secretos no sean vacíos, conocidos o demasiado cortos.
// En producción devuelve error (la API no debe arrancar); en desarrollo solo avisa.
func (c *Config) Validate() error {
	var problems []string
	problems = append(problems, checkSecret("JWT_SECRET", c.JWTSecret, minJWTSecretLen)...)
	problems = append(problems, checkSecret("INTERNAL_TOKEN", c.InternalToken, minInternalTokenLen)...)
	// Credenciales de MediaMTX: opcionales (si faltan, solo el INTERNAL_TOKEN puede publicar/leer),
	// pero si se definen deben ser robustas.
	if c.MediaPublishToken != "" {
		problems = append(problems, checkSecret("MEDIA_PUBLISH_TOKEN", c.MediaPublishToken, minInternalTokenLen)...)
	}
	if c.MediaReadToken != "" {
		problems = append(problems, checkSecret("MEDIA_READ_TOKEN", c.MediaReadToken, minInternalTokenLen)...)
	}

	if len(problems) == 0 {
		return nil
	}
	if c.IsDev() {
		for _, p := range problems {
			log.Printf("[SECURITY][DEV] %s (permitido solo porque APP_ENV=%s)", p, c.AppEnv)
		}
		if c.JWTSecret == "" {
			c.JWTSecret = "dev-only-insecure-jwt-secret-do-not-use-in-prod"
		}
		if c.InternalToken == "" {
			c.InternalToken = "dev-only-insecure-internal-token"
		}
		return nil
	}
	return fmt.Errorf("configuración insegura, la API no arrancará:\n  - %s\n(genera valores con `openssl rand -base64 48` o usa APP_ENV=dev en local)",
		strings.Join(problems, "\n  - "))
}

func checkSecret(name, value string, minLen int) []string {
	switch {
	case value == "":
		return []string{fmt.Sprintf("%s no está definido", name)}
	case knownInsecureSecrets[value]:
		return []string{fmt.Sprintf("%s usa un valor por defecto publicado en el repositorio", name)}
	case len(value) < minLen:
		return []string{fmt.Sprintf("%s es demasiado corto (mínimo %d caracteres)", name, minLen)}
	}
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}
