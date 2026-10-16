package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bigbrother/api/internal/alerts"
	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AlertHandler struct {
	db          *gorm.DB
	hub         *alerts.Hub
	cfg         *config.Config
	storageDir  string
	cooldowns   map[string]time.Time
	cooldownSec time.Duration
	mu          sync.Mutex
}

func NewAlertHandler(db *gorm.DB, hub *alerts.Hub, cfg *config.Config) *AlertHandler {
	storageDir := "./data/alerts"
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		fmt.Printf("Aviso: no se pudo crear el directorio de alertas %s: %v\n", storageDir, err)
	}

	return &AlertHandler{
		db:          db,
		hub:         hub,
		cfg:         cfg,
		storageDir:  storageDir,
		cooldowns:   make(map[string]time.Time),
		cooldownSec: 15 * time.Second, // 15 segundos entre eventos de la misma cámara para evitar inundación
	}
}

// Create ingesta alertas desde detectores locales (motion daemon, scripts o webcams)
func (h *AlertHandler) Create(c *gin.Context) {
	// Comprobar autenticación: JWT válido O Internal Token en header/query/form
	internalToken := c.GetHeader("X-Internal-Token")
	if internalToken == "" {
		internalToken = c.Query("token")
	}
	if internalToken == "" {
		internalToken = c.PostForm("token")
	}

	_, hasAuthUser := c.Get("userID")
	if !hasAuthUser && internalToken != h.cfg.InternalToken {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Message: "No autorizado para registrar alertas de seguridad",
			Error:   "UnauthorizedAlertSource",
		})
		return
	}

	cameraName := strings.TrimSpace(c.DefaultPostForm("camera_name", "camera"))
	if cameraName == "" {
		cameraName = "camera"
	}

	label := strings.TrimSpace(c.DefaultPostForm("label", "motion"))
	severity := strings.TrimSpace(c.DefaultPostForm("severity", "warning"))

	confidence := 1.0
	if confStr := c.PostForm("confidence"); confStr != "" {
		if val, err := strconv.ParseFloat(confStr, 64); err == nil {
			confidence = val
		}
	}

	// Smart Cooldown por cámara
	now := time.Now()
	h.mu.Lock()
	lastAlert, exists := h.cooldowns[cameraName]
	if exists && now.Sub(lastAlert) < h.cooldownSec {
		h.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"status":  "skipped",
			"message": fmt.Sprintf("Alerta omitida por cooldown activo para '%s'", cameraName),
		})
		return
	}
	h.cooldowns[cameraName] = now
	h.mu.Unlock()

	// Procesar imagen si viene adjunta
	var snapshotPath string
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		file, header, err = c.Request.FormFile("file")
	}

	if err == nil && file != nil {
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == "" || ext == "." {
			ext = ".jpg"
		}

		filename := fmt.Sprintf("alert_%d_%s%s", now.UnixNano(), strings.ReplaceAll(cameraName, " ", "_"), ext)
		diskPath := filepath.Join(h.storageDir, filename)

		out, err := os.Create(diskPath)
		if err == nil {
			defer out.Close()
			_, _ = io.Copy(out, file)
			snapshotPath = "/api/alerts/snapshots/" + filename
		}
	}

	// Buscar CameraID asociada si existe en base de datos
	var cameraID *uint
	var cam models.Camera
	if err := h.db.Where("name = ?", cameraName).First(&cam).Error; err == nil {
		cameraID = &cam.ID
	}

	alert := models.MotionAlert{
		CameraID:     cameraID,
		CameraName:   cameraName,
		SnapshotPath: snapshotPath,
		DetectedAt:   now,
		Label:        label,
		Confidence:   confidence,
		Acknowledged: false,
		Severity:     severity,
	}

	if err := h.db.Create(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al registrar la alerta de movimiento en base de datos",
			Error:   err.Error(),
		})
		return
	}

	// Difundir inmediatamente a todos los clientes SSE conectados (Telescreen)
	h.hub.Broadcast(&alert)

	c.JSON(http.StatusCreated, alert)
}

// GetAll lista las alertas históricas con paginación y filtros
func (h *AlertHandler) GetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := h.db.Model(&models.MotionAlert{})

	if cam := c.Query("camera"); cam != "" {
		query = query.Where("camera_name = ?", cam)
	}

	if ack := c.Query("acknowledged"); ack != "" {
		if ack == "true" {
			query = query.Where("acknowledged = true")
		} else if ack == "false" {
			query = query.Where("acknowledged = false")
		}
	}

	var total int64
	query.Count(&total)

	var alerts []models.MotionAlert
	if err := query.Order("detected_at DESC").Limit(limit).Offset(offset).Find(&alerts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al recuperar alertas",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

// GetUnreadCount devuelve el número de alertas sin revisar
func (h *AlertHandler) GetUnreadCount(c *gin.Context) {
	var count int64
	h.db.Model(&models.MotionAlert{}).Where("acknowledged = false").Count(&count)
	c.JSON(http.StatusOK, gin.H{"unreadCount": count})
}

// Acknowledge marca una alerta como leída / atendida
func (h *AlertHandler) Acknowledge(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Model(&models.MotionAlert{}).Where("id = ?", id).Update("acknowledged", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al marcar la alerta como atendida",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged", "id": id})
}

// AcknowledgeAll marca todas las alertas pendientes como atendidas
func (h *AlertHandler) AcknowledgeAll(c *gin.Context) {
	if err := h.db.Model(&models.MotionAlert{}).Where("acknowledged = false").Update("acknowledged", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al marcar todas las alertas como atendidas",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "all_acknowledged"})
}

// Delete elimina una alerta y su captura de disco
func (h *AlertHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var alert models.MotionAlert
	if err := h.db.First(&alert, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: "Alerta no encontrada",
			Error:   err.Error(),
		})
		return
	}

	// Borrar fichero de snapshot si existe
	if alert.SnapshotPath != "" {
		filename := filepath.Base(alert.SnapshotPath)
		diskPath := filepath.Join(h.storageDir, filename)
		_ = os.Remove(diskPath)
	}

	if err := h.db.Delete(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al eliminar la alerta",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": id})
}

// ClearAll elimina todo el historial de alertas
func (h *AlertHandler) ClearAll(c *gin.Context) {
	var alerts []models.MotionAlert
	h.db.Find(&alerts)
	for _, a := range alerts {
		if a.SnapshotPath != "" {
			_ = os.Remove(filepath.Join(h.storageDir, filepath.Base(a.SnapshotPath)))
		}
	}
	h.db.Exec("DELETE FROM alerts")
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}

// Stream proporciona conexión Server-Sent Events (SSE) para emisión en tiempo real
func (h *AlertHandler) Stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	clientChan := h.hub.Register()
	defer h.hub.Unregister(clientChan)

	// Heartbeat inicial para confirmar conexión al frontend
	c.Writer.WriteString("event: connected\ndata: {\"status\":\"connected\"}\n\n")
	c.Writer.Flush()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return

		case <-ticker.C:
			// Ping periódico para mantener vivos los proxies y evitar timeouts
			c.Writer.WriteString(": ping\n\n")
			c.Writer.Flush()

		case alert, ok := <-clientChan:
			if !ok {
				return
			}
			data, err := json.Marshal(alert)
			if err == nil {
				c.Writer.WriteString("event: alert\n")
				c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", data))
				c.Writer.Flush()
			}
		}
	}
}
