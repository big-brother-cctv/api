package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	validUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{3,32}$`)
	validEmailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

type AuthHandler struct {
	db         *gorm.DB
	jwtService *auth.JWTService
}

func NewAuthHandler(db *gorm.DB, jwtService *auth.JWTService) *AuthHandler {
	return &AuthHandler{db: db, jwtService: jwtService}
}

type AuthRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Cuerpo de solicitud inválido",
			Error:   err.Error(),
		})
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "El nombre de usuario y la contraseña son obligatorios",
			Error:   "MissingCredentials",
		})
		return
	}

	var user models.User
	if err := h.db.Where("LOWER(username) = LOWER(?)", username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Message: "Credenciales de acceso inválidas",
			Error:   "InvalidCredentials",
		})
		return
	}

	// Verificar si el usuario está deshabilitado
	if user.Disabled {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Message: "La cuenta de usuario se encuentra suspendida o deshabilitada",
			Error:   "AccountDisabled",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Message: "Credenciales de acceso inválidas",
			Error:   "InvalidCredentials",
		})
		return
	}

	role := user.Role
	if role == "" {
		if strings.EqualFold(user.Username, "admin") {
			role = "admin"
		} else {
			role = "operator"
		}
	}

	token, err := h.jwtService.GenerateToken(user.Username, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al generar el token de sesión",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token})
}

type RegisterRequest struct {
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required"`
	Password       string `json:"password"`
	HashedPassword string `json:"hashedPassword"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	Permissions    string `json:"permissions"`
	Photo          string `json:"photo"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Cuerpo de solicitud inválido",
			Error:   err.Error(),
		})
		return
	}

	pass := strings.TrimSpace(req.Password)
	if pass == "" {
		pass = strings.TrimSpace(req.HashedPassword)
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)

	if username == "" || email == "" || pass == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "El nombre de usuario, correo electrónico y contraseña son obligatorios",
			Error:   "MissingFields",
		})
		return
	}

	if !validUsernameRegex.MatchString(username) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "El nombre de usuario debe tener entre 3 y 32 caracteres alfanuméricos (letras, números, '.', '_', '-')",
			Error:   "InvalidUsernameFormat",
		})
		return
	}

	if !validEmailRegex.MatchString(email) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Formato de correo electrónico inválido",
			Error:   "InvalidEmailFormat",
		})
		return
	}

	if len(pass) < 6 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "La contraseña debe contener al menos 6 caracteres",
			Error:   "WeakPassword",
		})
		return
	}

	// Comprobar si ya existe username
	var existingUser models.User
	if err := h.db.Where("LOWER(username) = LOWER(?)", username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Message: "El nombre de usuario ya está registrado",
			Error:   "UsernameAlreadyExists",
		})
		return
	}

	// Comprobar si ya existe email
	if err := h.db.Where("LOWER(email) = LOWER(?)", email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Message: "El correo electrónico ya está registrado",
			Error:   "EmailAlreadyExists",
		})
		return
	}

	// Hashear contraseña con bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al procesar la contraseña",
			Error:   err.Error(),
		})
		return
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "operator"
	}

	newUser := models.User{
		Username:       username,
		Email:          email,
		HashedPassword: string(hashedBytes),
		Name:           strings.TrimSpace(req.Name),
		Role:           role,
		Permissions:    req.Permissions,
		Photo:          req.Photo,
		Disabled:       false,
	}

	if err := h.db.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al registrar el usuario en el sistema",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Usuario registrado correctamente",
		"user":    newUser.ToDTO(),
	})
}
