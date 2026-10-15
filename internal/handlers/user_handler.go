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
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid user payload",
			Error:   err.Error(),
		})
		return
	}

	if user.HashedPassword != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(user.HashedPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Message: "Failed to hash password",
				Error:   err.Error(),
			})
			return
		}
		user.HashedPassword = string(hashed)
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Could not create user",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, user.ToDTO())
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

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid update payload",
			Error:   err.Error(),
		})
		return
	}

	if v, ok := input["name"].(string); ok {
		user.Name = v
	}
	if v, ok := input["email"].(string); ok {
		user.Email = v
	}
	if v, ok := input["username"].(string); ok && strings.TrimSpace(v) != "" {
		user.Username = strings.TrimSpace(v)
	}
	if v, ok := input["photo"].(string); ok {
		user.Photo = v
	}

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Failed to update user",
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

	if err := h.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Failed to delete user",
			Error:   err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
