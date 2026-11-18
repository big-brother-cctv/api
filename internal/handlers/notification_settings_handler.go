package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/bigbrother/api/internal/models"
	"github.com/bigbrother/api/internal/notify"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NotificationSettingsHandler struct {
	db *gorm.DB
}

func NewNotificationSettingsHandler(db *gorm.DB) *NotificationSettingsHandler {
	return &NotificationSettingsHandler{db: db}
}

func settingsResponse(s notify.Settings) gin.H {
	return gin.H{"settings": s, "classes": notify.Classes}
}

// Get devuelve el modo actual, las reglas y las clases configurables.
func (h *NotificationSettingsHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, settingsResponse(notify.Load(h.db)))
}

// SetMode cambia entre "home" y "away" (cualquier usuario: armar/desarmar al salir de casa).
func (h *NotificationSettingsHandler) SetMode(c *gin.Context) {
	if c.GetBool("is_internal") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Message: "Los tokens internos no cambian el modo", Error: "Forbidden"})
		return
	}
	var in struct {
		Mode notify.Mode `json:"mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Falta el modo", Error: err.Error()})
		return
	}
	s := notify.Load(h.db)
	s.Mode = in.Mode
	h.save(c, s)
}

// UpdateRules cambia qué clases avisan en cada modo (admin/operator).
func (h *NotificationSettingsHandler) UpdateRules(c *gin.Context) {
	var in struct {
		Home []string `json:"home"`
		Away []string `json:"away"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Home == nil || in.Away == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Se esperan las listas 'home' y 'away'", Error: "InvalidRules"})
		return
	}
	s := notify.Load(h.db)
	s.Home, s.Away = in.Home, in.Away
	h.save(c, s)
}

func (h *NotificationSettingsHandler) save(c *gin.Context, s notify.Settings) {
	by := c.GetString("username")
	saved, err := notify.Save(h.db, s, by, time.Now())
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, notify.ErrInvalidSettings) {
			status = http.StatusBadRequest
		}
		c.JSON(status, models.ErrorResponse{Message: "No se pudo guardar la configuración de notificaciones", Error: err.Error()})
		return
	}
	log.Printf("[NOTIFY] %s: modo=%s casa=%v fuera=%v", by, saved.Mode, saved.Home, saved.Away)
	c.JSON(http.StatusOK, settingsResponse(saved))
}
