package handlers

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/mediamtx"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var validCameraNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{2,48}$`)

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
	if cam.MotionThreshold == 0 {
		cam.MotionThreshold = 350
	}
	if cam.MotionFrames == 0 {
		cam.MotionFrames = 2
	}
	if cam.MotionCooldown == 0 {
		cam.MotionCooldown = 15
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

	activePaths, _ := h.mtxService.GetActivePaths()

	for i := range cameras {
		h.enrichCamera(&cameras[i])
		if activePaths != nil {
			if isReady, exists := activePaths[cameras[i].Name]; exists && isReady {
				cameras[i].Status = "online"
			} else if isReady, exists := activePaths[strings.ToLower(cameras[i].Name)]; exists && isReady {
				cameras[i].Status = "online"
			} else {
				cameras[i].Status = "standby"
			}
		}
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
			Message: "El nombre de la cámara es obligatorio",
			Error:   "CameraNameRequired",
		})
		return
	}
	if !validCameraNameRegex.MatchString(trimmedName) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "El nombre de la cámara debe contener entre 2 y 48 caracteres alfanuméricos, guiones o puntos (sin espacios)",
			Error:   "InvalidCameraNameFormat",
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

	if camera.FPS == "" {
		camera.FPS = "30"
	}
	if camera.Resolution == "" {
		camera.Resolution = "1280x720"
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
		newName := strings.TrimSpace(v)
		if !validCameraNameRegex.MatchString(newName) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Message: "El nombre de la cámara debe contener entre 2 y 48 caracteres alfanuméricos, guiones o puntos (sin espacios)",
				Error:   "InvalidCameraNameFormat",
			})
			return
		}
		if !strings.EqualFold(newName, camera.Name) {
			var collision models.Camera
			if err := h.db.Where("LOWER(name) = LOWER(?) AND id != ?", newName, camera.ID).First(&collision).Error; err == nil {
				c.JSON(http.StatusConflict, models.ErrorResponse{
					Message: fmt.Sprintf("Ya existe otra cámara con el nombre '%s'", newName),
					Error:   "DuplicateCameraName",
				})
				return
			}
		}
		camera.Name = newName
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
	if v, ok := input["motionEnabled"].(bool); ok {
		camera.MotionEnabled = v
	}
	if v, ok := input["motionThreshold"].(float64); ok {
		camera.MotionThreshold = int(v)
	}
	if v, ok := input["motionFrames"].(float64); ok {
		camera.MotionFrames = int(v)
	}
	if v, ok := input["motionCooldown"].(float64); ok {
		camera.MotionCooldown = int(v)
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
			fps = "30"
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
		log.Printf("[CCTV HEARTBEAT] Nueva cámara auto-registrada: '%s'", camera.Name)
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

// GetMotionConfig genera la configuración unificada de todas las cámaras para el daemon Motion
func (h *CameraHandler) GetMotionConfig(c *gin.Context) {
	var cameras []models.Camera
	if err := h.db.Find(&cameras).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error fetching motion configuration",
			Error:   err.Error(),
		})
		return
	}

	type MotionCameraItem struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		RTSPURL   string `json:"rtspUrl"`
		Threshold int    `json:"threshold"`
		Frames    int    `json:"frames"`
		Cooldown  int    `json:"cooldown"`
		Enabled   bool   `json:"enabled"`
	}

	var items []MotionCameraItem
	for _, cam := range cameras {
		threshold := cam.MotionThreshold
		if threshold == 0 {
			threshold = 350
		}
		frames := cam.MotionFrames
		if frames == 0 {
			frames = 2
		}
		cooldown := cam.MotionCooldown
		if cooldown == 0 {
			cooldown = 15
		}
		items = append(items, MotionCameraItem{
			ID:        cam.ID,
			Name:      cam.Name,
			RTSPURL:   fmt.Sprintf("rtsp://%s:8554/%s", h.cfg.MediaMTXRTSPHost, cam.Name),
			Threshold: threshold,
			Frames:    frames,
			Cooldown:  cooldown,
			Enabled:   cam.MotionEnabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"webcontrol_port": 8083,
		"cameras":         items,
	})
}
