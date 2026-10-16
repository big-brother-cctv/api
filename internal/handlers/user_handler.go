package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

func (h *UserHandler) GetAll(c *gin.Context) {
	var users []models.User
	if err := h.db.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error fetching users",
			Error:   err.Error(),
		})
		return
	}

	dtos := make([]models.UserDTO, len(users))
	for i, u := range users {
		dtos[i] = u.ToDTO()
	}
	c.JSON(http.StatusOK, dtos)
}

func (h *UserHandler) Create(c *gin.Context) {
	var input struct {
		Username    string `json:"username" binding:"required"`
		Email       string `json:"email" binding:"required"`
		Password    string `json:"password" binding:"required"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		Permissions string `json:"permissions"`
		Photo       string `json:"photo"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Payload de usuario inválido",
			Error:   err.Error(),
		})
		return
	}

	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "operator"
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Fallo al procesar credencial de acceso",
			Error:   err.Error(),
		})
		return
	}

	user := models.User{
		Username:       strings.TrimSpace(input.Username),
		Email:          strings.TrimSpace(input.Email),
		HashedPassword: string(hashed),
		Name:           strings.TrimSpace(input.Name),
		Role:           role,
		Permissions:    input.Permissions,
		Photo:          input.Photo,
		Disabled:       false,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "No se pudo registrar al camarada en el censo",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, user.ToDTO())
}

func (h *UserHandler) GetMe(c *gin.Context) {
	usernameVal, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Message: "User not authenticated",
			Error:   "Unauthorized",
		})
		return
	}

	username, ok := usernameVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Invalid authentication state",
			Error:   "Server error",
		})
		return
	}

	var user models.User
	if err := h.db.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("User not found with username: %s", username),
			Error:   "Resource not found",
		})
		return
	}

	c.JSON(http.StatusOK, user.ToDTO())
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid user ID",
			Error:   "Invalid ID",
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("User not found with id: %d", id),
			Error:   "Resource not found",
		})
		return
	}

	c.JSON(http.StatusOK, user.ToDTO())
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Identificador de usuario inválido",
			Error:   "Invalid ID",
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Camarada no encontrado con id: %d", id),
			Error:   "Resource not found",
		})
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Carga útil de actualización inválida",
			Error:   err.Error(),
		})
		return
	}

	if v, ok := input["name"].(string); ok {
		user.Name = v
	}
	if v, ok := input["email"].(string); ok && strings.TrimSpace(v) != "" {
		user.Email = strings.TrimSpace(v)
	}
	if v, ok := input["username"].(string); ok && strings.TrimSpace(v) != "" {
		user.Username = strings.TrimSpace(v)
	}
	if v, ok := input["role"].(string); ok && strings.TrimSpace(v) != "" {
		user.Role = strings.TrimSpace(v)
	}
	if v, ok := input["permissions"].(string); ok {
		user.Permissions = v
	}
	if v, ok := input["disabled"].(bool); ok {
		user.Disabled = v
	}
	if v, ok := input["photo"].(string); ok {
		user.Photo = v
	}
	if v, ok := input["password"].(string); ok && strings.TrimSpace(v) != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(v)), bcrypt.DefaultCost)
		if err == nil {
			user.HashedPassword = string(hashed)
		}
	}

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Fallo al actualizar directivas del camarada",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, user.ToDTO())
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Identificador de usuario inválido",
			Error:   "Invalid ID",
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Usuario no encontrado con id: %d", id),
			Error:   "Resource not found",
		})
		return
	}

	// 1. Protección contra auto-vaporización
	currentAuthUser := c.GetString("username")
	if user.Username == currentAuthUser {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Un miembro del Partido no puede vaporizarse a sí mismo.",
			Error:   "SelfDeletionForbidden",
		})
		return
	}

	// 2. Protección contra eliminar al último administrador
	if user.Role == "admin" || user.Username == "admin" {
		var adminCount int64
		h.db.Model(&models.User{}).Where("role = ? OR username = ?", "admin", "admin").Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Message: "No es posible eliminar al último administrador supremo de Oceanía.",
				Error:   "LastAdminDeletionForbidden",
			})
			return
		}
	}

	if err := h.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Fallo al vaporizar al usuario",
			Error:   err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
