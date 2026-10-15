package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/mediamtx"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CameraHandler struct {
	db         *gorm.DB
	mtxService *mediamtx.Service
	cfg        *config.Config
}

func NewCameraHandler(db *gorm.DB, mtxService *mediamtx.Service, cfg *config.Config) *CameraHandler {
	return &CameraHandler{
		db:         db,
		mtxService: mtxService,
		cfg:        cfg,
	}
}

func (h *CameraHandler) enrichCamera(cam *models.Camera) {
	cam.StreamURL = fmt.Sprintf("/stream/%s/index.m3u8", cam.Name)
	cam.IngestURL = fmt.Sprintf("rtsp://%s:8554/%s", h.cfg.MediaMTXRTSPHost, cam.Name)
	if cam.ConnectionMode == "" {
		if cam.SourceType == "rtsp" || cam.SourceType == "http" {
			if cam.SourceURL != "" {
				cam.ConnectionMode = "pull"
			} else {
				cam.ConnectionMode = "push"
			}
		} else if cam.SourceType == "test" {
			cam.ConnectionMode = "test"
		} else {
			cam.ConnectionMode = "local"
		}
	}
	if cam.Status == "" {
		cam.Status = "offline"
	}
}

func (h *CameraHandler) GetAll(c *gin.Context) {
	var cameras []models.Camera
	if err := h.db.Find(&cameras).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error fetching cameras",
			Error:   err.Error(),
		})
		return
	}
	if cameras == nil {
		cameras = []models.Camera{}
	}
	for i := range cameras {
		h.enrichCamera(&cameras[i])
	}
	c.JSON(http.StatusOK, cameras)
}

func (h *CameraHandler) Create(c *gin.Context) {
	var camera models.Camera
	if err := c.ShouldBindJSON(&camera); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid request payload",
			Error:   err.Error(),
		})
		return
	}

	trimmedName := strings.TrimSpace(camera.Name)
	if trimmedName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Camera name is required",
			Error:   "Invalid camera name",
		})
		return
	}
	camera.Name = trimmedName

	var existing models.Camera
	if err := h.db.Where("LOWER(name) = LOWER(?)", trimmedName).First(&existing).Error; err == nil {
		msg := fmt.Sprintf(
			"Camera name '%s' already exists (id: %d, device: %s, resolution: %s, fps: %s)",
			existing.Name, existing.ID, existing.Device, existing.Resolution, existing.FPS,
		)
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Message: msg,
			Error:   "Duplicate camera name",
		})
		return
	}

	if camera.SourceType == "" {
		if strings.HasPrefix(camera.SourceURL, "rtsp://") || strings.HasPrefix(camera.Device, "rtsp://") {
			camera.SourceType = "rtsp"
		} else if strings.HasPrefix(camera.SourceURL, "http://") || strings.HasPrefix(camera.SourceURL, "https://") {
			camera.SourceType = "http"
		} else if camera.Device == "test" || camera.Device == "virtual" || camera.SourceURL == "test" {
			camera.SourceType = "test"
		} else {
			camera.SourceType = "v4l2"
		}
	}

	if camera.ConnectionMode == "" {
		if (camera.SourceType == "rtsp" || camera.SourceType == "http") && camera.SourceURL != "" {
			camera.ConnectionMode = "pull"
		} else if camera.SourceType == "test" {
			camera.ConnectionMode = "test"
		} else if camera.SourceType == "v4l2" {
			camera.ConnectionMode = "local"
		} else {
			camera.ConnectionMode = "push"
		}
	}

	if camera.Status == "" {
		camera.Status = "standby"
	}

	if err := h.db.Create(&camera).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Could not create camera",
			Error:   err.Error(),
		})
		return
	}

	// Si es una cámara IP remota en modo PULL, configuramos MediaMTX para que sintonice de inmediato
	if (camera.ConnectionMode == "pull" || camera.SourceType == "rtsp" || camera.SourceType == "http") && camera.SourceURL != "" {
		if err := h.mtxService.ConfigurePullPath(camera.Name, camera.SourceURL); err != nil {
			log.Printf("[MediaMTX] Warning: error al registrar ruta PULL para '%s': %v", camera.Name, err)
		}
	}

	h.enrichCamera(&camera)
	c.JSON(http.StatusCreated, camera)
}

func (h *CameraHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid camera ID",
			Error:   "Invalid ID",
		})
		return
	}

	var camera models.Camera
	if err := h.db.First(&camera, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Camera not found with id: %d", id),
			Error:   "Resource not found",
		})
		return
	}

	h.enrichCamera(&camera)
	c.JSON(http.StatusOK, camera)
}

func (h *CameraHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid camera ID",
			Error:   "Invalid ID",
		})
		return
	}

	var camera models.Camera
	if err := h.db.First(&camera, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Camera not found with id: %d", id),
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

	// Actualización selectiva de campos presentes en el payload
	if v, ok := input["name"].(string); ok && strings.TrimSpace(v) != "" {
		camera.Name = strings.TrimSpace(v)
	}
	if v, ok := input["sourceType"].(string); ok {
		camera.SourceType = v
	}
	if v, ok := input["connectionMode"].(string); ok {
		camera.ConnectionMode = v
	}
	if v, ok := input["sourceUrl"].(string); ok {
		camera.SourceURL = v
	}
	if v, ok := input["device"].(string); ok {
		camera.Device = v
	}
	if v, ok := input["resolution"].(string); ok {
		camera.Resolution = v
	}
	if v, ok := input["fps"].(string); ok {
		camera.FPS = v
	}
	if v, ok := input["postUrl"].(string); ok {
		camera.PostURL = v
	}
	if v, ok := input["codec"].(string); ok {
		camera.Codec = v
	}
	if v, ok := input["preset"].(string); ok {
		camera.Preset = v
	}
	if v, ok := input["tune"].(string); ok {
		camera.Tune = v
	}
	if v, ok := input["buffer"].(string); ok {
		camera.Buffer = v
	}
	if v, ok := input["rotation"].(string); ok {
		camera.Rotation = v
	}

	if err := h.db.Save(&camera).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Failed to update camera",
			Error:   err.Error(),
		})
		return
	}

	// Sincronizar con MediaMTX si es modo PULL
	if (camera.ConnectionMode == "pull" || camera.SourceType == "rtsp" || camera.SourceType == "http") && camera.SourceURL != "" {
		if err := h.mtxService.ConfigurePullPath(camera.Name, camera.SourceURL); err != nil {
			log.Printf("[MediaMTX] Warning: error al actualizar ruta PULL para '%s': %v", camera.Name, err)
		}
	}

	h.enrichCamera(&camera)
	c.JSON(http.StatusOK, camera)
}

func (h *CameraHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid camera ID",
			Error:   "Invalid ID",
		})
		return
	}

	var camera models.Camera
	if err := h.db.First(&camera, id).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Message: fmt.Sprintf("Camera not found with id: %d", id),
			Error:   "Resource not found",
		})
		return
	}

	// Remover ruta en MediaMTX si existía
	_ = h.mtxService.RemovePath(camera.Name)

	if err := h.db.Delete(&camera).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Failed to delete camera",
			Error:   err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// Heartbeat maneja las señales periódicas de vida y auto-registro de agentes de cámara remotos
func (h *CameraHandler) Heartbeat(c *gin.Context) {
	var req models.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Invalid heartbeat payload",
			Error:   err.Error(),
		})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Camera name is required",
			Error:   "Invalid name",
		})
		return
	}

	now := time.Now()
	var camera models.Camera
	if err := h.db.Where("LOWER(name) = LOWER(?)", name).First(&camera).Error; err != nil {
		// La cámara no existía: la auto-registramos (Auto-Handshake)
		mode := req.ConnectionMode
		if mode == "" {
			mode = "push"
		}
		sourceType := req.SourceType
		if sourceType == "" {
			sourceType = "v4l2"
		}
		resolution := req.Resolution
		if resolution == "" {
			resolution = "1280x720"
		}
		fps := req.FPS
		if fps == "" {
			fps = "25"
		}

		camera = models.Camera{
			Name:           name,
			SourceType:     sourceType,
			ConnectionMode: mode,
			Device:         req.Device,
			Resolution:     resolution,
			FPS:            fps,
			Codec:          "libx264",
			Preset:         "ultrafast",
			Tune:           "zerolatency",
			Status:         "online",
			LastSeen:       &now,
		}
		if err := h.db.Create(&camera).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Message: "Failed to auto-register camera via heartbeat",
				Error:   err.Error(),
			})
			return
		}
		log.Printf("[INGSOC HEARTBEAT] Nueva cámara auto-registrada: '%s'", camera.Name)
	} else {
		// Ya existía: actualizamos lastSeen y status
		updates := map[string]interface{}{
			"status":    "online",
			"last_seen": &now,
		}
		if req.Resolution != "" && (camera.Resolution == "" || camera.Resolution == "AUTO-DETECT") {
			updates["resolution"] = req.Resolution
			camera.Resolution = req.Resolution
		}
		if req.FPS != "" && (camera.FPS == "" || camera.FPS == "AUTO") {
			updates["fps"] = req.FPS
			camera.FPS = req.FPS
		}
		h.db.Model(&camera).Updates(updates)
		camera.Status = "online"
		camera.LastSeen = &now
	}

	h.enrichCamera(&camera)
	c.JSON(http.StatusOK, camera)
}

func (h *CameraHandler) Search(c *gin.Context) {
	name := c.Query("name")
	var cameras []models.Camera

	query := h.db
	if strings.TrimSpace(name) != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(name)+"%")
	}

	if err := query.Find(&cameras).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error searching cameras",
			Error:   err.Error(),
		})
		return
	}

	if cameras == nil {
		cameras = []models.Camera{}
	}
	c.JSON(http.StatusOK, cameras)
}
