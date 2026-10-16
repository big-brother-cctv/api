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
	if err := h.db.Order("id ASC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al obtener la lista de usuarios",
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
			Message: "Cuerpo de solicitud de usuario inválido",
			Error:   err.Error(),
		})
		return
	}

	username := strings.TrimSpace(input.Username)
	email := strings.TrimSpace(input.Email)
	password := strings.TrimSpace(input.Password)

	if username == "" || email == "" || password == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Nombre de usuario, email y contraseña son obligatorios",
			Error:   "MissingFields",
		})
		return
	}

	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "La contraseña debe tener al menos 6 caracteres",
			Error:   "WeakPassword",
		})
		return
	}

	// Comprobar si ya existe username o email
	var existing models.User
	if err := h.db.Where("LOWER(username) = LOWER(?)", username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Message: "El nombre de usuario ya se encuentra registrado",
			Error:   "UsernameAlreadyExists",
		})
		return
	}
	if err := h.db.Where("LOWER(email) = LOWER(?)", email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Message: "El correo electrónico ya se encuentra registrado",
			Error:   "EmailAlreadyExists",
		})
		return
	}

	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "operator"
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al procesar la contraseña del usuario",
			Error:   err.Error(),
		})
		return
	}

	user := models.User{
		Username:       username,
		Email:          email,
		HashedPassword: string(hashed),
		Name:           strings.TrimSpace(input.Name),
		Role:           role,
		Permissions:    input.Permissions,
		Photo:          input.Photo,
		Disabled:       false,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "No se pudo crear el usuario en la base de datos",
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
			Message: "Usuario no autenticado",
			Error:   "Unauthorized",
		})
		return
	}

	username, ok := usernameVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Estado de autenticación inválido",
			Error:   "ServerError",
		})
		return
	}

	var user models.User
	if err := h.db.Where("LOWER(username) = LOWER(?)", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Usuario no encontrado con username: %s", username),
			Error:   "NotFound",
		})
		return
	}

	c.JSON(http.StatusOK, user.ToDTO())
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Identificador de usuario inválido",
			Error:   "InvalidID",
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Usuario no encontrado con id: %d", id),
			Error:   "NotFound",
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
			Error:   "InvalidID",
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Usuario no encontrado con id: %d", id),
			Error:   "NotFound",
		})
		return
	}

	// Comprobación de seguridad: usuarios no admin solo pueden actualizarse a sí mismos
	currentRole := c.GetString("role")
	currentUsername := c.GetString("username")
	isAdmin := strings.EqualFold(currentRole, "admin") || strings.EqualFold(currentUsername, "admin")

	if !isAdmin && !strings.EqualFold(user.Username, currentUsername) {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Message: "No tiene permisos para modificar la cuenta de otro usuario",
			Error:   "Forbidden",
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
		user.Name = strings.TrimSpace(v)
	}
	if v, ok := input["email"].(string); ok && strings.TrimSpace(v) != "" {
		newEmail := strings.TrimSpace(v)
		// Si cambia el email, verificar colisión
		if !strings.EqualFold(newEmail, user.Email) {
			var collision models.User
			if err := h.db.Where("LOWER(email) = LOWER(?) AND id != ?", newEmail, user.ID).First(&collision).Error; err == nil {
				c.JSON(http.StatusConflict, models.ErrorResponse{
					Message: "El correo electrónico ya está en uso por otra cuenta",
					Error:   "EmailCollision",
				})
				return
			}
		}
		user.Email = newEmail
	}
	if v, ok := input["username"].(string); ok && strings.TrimSpace(v) != "" {
		newUsername := strings.TrimSpace(v)
		if !strings.EqualFold(newUsername, user.Username) {
			var collision models.User
			if err := h.db.Where("LOWER(username) = LOWER(?) AND id != ?", newUsername, user.ID).First(&collision).Error; err == nil {
				c.JSON(http.StatusConflict, models.ErrorResponse{
					Message: "El nombre de usuario ya está en uso por otra cuenta",
					Error:   "UsernameCollision",
				})
				return
			}
		}
		user.Username = newUsername
	}

	// Campos restringidos únicamente a administradores
	if isAdmin {
		if v, ok := input["role"].(string); ok && strings.TrimSpace(v) != "" {
			user.Role = strings.TrimSpace(v)
		}
		if v, ok := input["permissions"].(string); ok {
			user.Permissions = v
		}
		if v, ok := input["disabled"].(bool); ok {
			// Prevenir deshabilitar al último administrador
			if v && (user.Role == "admin" || user.Username == "admin") {
				var adminCount int64
				h.db.Model(&models.User{}).Where("(role = ? OR username = ?) AND disabled = ?", "admin", "admin", false).Count(&adminCount)
				if adminCount <= 1 {
					c.JSON(http.StatusBadRequest, models.ErrorResponse{
						Message: "No es posible deshabilitar al único administrador activo del sistema",
						Error:   "LastAdminCannotBeDisabled",
					})
					return
				}
			}
			user.Disabled = v
		}
	}

	if v, ok := input["photo"].(string); ok {
		user.Photo = v
	}

	if v, ok := input["password"].(string); ok && strings.TrimSpace(v) != "" {
		pass := strings.TrimSpace(v)
		if len(pass) < 6 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Message: "La contraseña debe tener al menos 6 caracteres",
				Error:   "WeakPassword",
			})
			return
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err == nil {
			user.HashedPassword = string(hashed)
		}
	}

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al actualizar los datos del usuario",
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
			Error:   "InvalidID",
		})
		return
	}

	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Usuario no encontrado con id: %d", id),
			Error:   "NotFound",
		})
		return
	}

	// 1. Protección contra auto-eliminación
	currentAuthUser := c.GetString("username")
	if strings.EqualFold(user.Username, currentAuthUser) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "No es posible eliminar la propia cuenta de usuario en sesión.",
			Error:   "SelfDeletionForbidden",
		})
		return
	}

	// 2. Protección contra eliminar al último administrador
	if user.Role == "admin" || strings.EqualFold(user.Username, "admin") {
		var adminCount int64
		h.db.Model(&models.User{}).Where("role = ? OR username = ?", "admin", "admin").Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Message: "No es posible eliminar al último administrador del sistema.",
				Error:   "LastAdminDeletionForbidden",
			})
			return
		}
	}

	if err := h.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al eliminar el usuario del sistema",
			Error:   err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
