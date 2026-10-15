package handlers

import (
	"net/http"

	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
		c.String(http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.String(http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		c.String(http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := h.jwtService.GenerateToken(user.Username)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate token")
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
	Photo          string `json:"photo"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Invalid request body")
		return
	}

	pass := req.Password
	if pass == "" {
		pass = req.HashedPassword
	}

	if req.Username == "" || req.Email == "" || pass == "" {
		c.String(http.StatusBadRequest, "Username, email and password are required")
		return
	}

	// Comprobar si ya existe username
	var existingUser models.User
	if err := h.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.String(http.StatusConflict, "Username already exists")
		return
	}

	// Comprobar si ya existe email
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.String(http.StatusConflict, "Email already exists")
		return
	}

	// Hashear contraseña con bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to hash password")
		return
	}

	newUser := models.User{
		Username:       req.Username,
		Email:          req.Email,
		HashedPassword: string(hashedBytes),
		Name:           req.Name,
		Photo:          req.Photo,
		Disabled:       false,
	}

	if err := h.db.Create(&newUser).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to create user")
		return
	}

	c.String(http.StatusCreated, "User registered successfully")
}
