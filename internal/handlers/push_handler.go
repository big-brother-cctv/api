package handlers

import (
	"errors"
	"net/http"

	"github.com/bigbrother/api/internal/models"
	"github.com/bigbrother/api/internal/push"
	"github.com/gin-gonic/gin"
)

type PushHandler struct {
	svc *push.Service
}

func NewPushHandler(svc *push.Service) *PushHandler {
	return &PushHandler{svc: svc}
}

// PublicKey devuelve la clave VAPID pública (applicationServerKey).
func (h *PushHandler) PublicKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"publicKey": h.svc.PublicKey()})
}

// Subscribe registra la suscripción Web Push del navegador del usuario autenticado.
func (h *PushHandler) Subscribe(c *gin.Context) {
	if c.GetBool("is_internal") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Message: "Los tokens internos no pueden suscribirse", Error: "Forbidden"})
		return
	}
	var in push.SubscriptionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Suscripción inválida", Error: err.Error()})
		return
	}
	if err := h.svc.Subscribe(c.GetString("username"), in, c.GetHeader("User-Agent")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, push.ErrInvalidSubscription) {
			status = http.StatusBadRequest
		}
		c.JSON(status, models.ErrorResponse{Message: "No se pudo registrar la suscripción push", Error: err.Error()})
		return
	}
	c.Status(http.StatusCreated)
}

// Unsubscribe elimina la suscripción del endpoint indicado (solo si es del usuario).
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	var in struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Falta el endpoint", Error: err.Error()})
		return
	}
	if err := h.svc.Unsubscribe(c.GetString("username"), in.Endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "No se pudo eliminar la suscripción", Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// Test envía una notificación de prueba a los dispositivos del usuario.
func (h *PushHandler) Test(c *gin.Context) {
	n, err := h.svc.NotifyUser(c.GetString("username"), push.Payload{
		Title: "✅ Big Brother CCTV",
		Body:  "Las notificaciones push funcionan en este dispositivo.",
		Tag:   "bb-push-test",
		Data:  map[string]any{"url": "/?view=alerts"},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Error enviando la prueba", Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"devices": n})
}
