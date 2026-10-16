package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
)

var (
	allowedImageExts = map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".svg":  true,
		".gif":  true,
	}

	allowedVideoExts = map[string]bool{
		".mp4":  true,
		".mkv":  true,
		".webm": true,
		".avi":  true,
		".mov":  true,
	}
)

type UploadHandler struct {
	cfg *config.Config
}

func NewUploadHandler(cfg *config.Config) *UploadHandler {
	return &UploadHandler{cfg: cfg}
}

func (h *UploadHandler) UploadImage(c *gin.Context) {
	h.proxyUpload(c, "images", allowedImageExts, 20*1024*1024)
}

func (h *UploadHandler) UploadVideo(c *gin.Context) {
	h.proxyUpload(c, "videos", allowedVideoExts, 100*1024*1024)
}

func (h *UploadHandler) proxyUpload(c *gin.Context, folder string, allowedExts map[string]bool, maxBytes int64) {
	// Limitar el tamaño de lectura para evitar ataques de denegación de servicio por archivos gigantescos
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "No se proporcionó ningún archivo válido o el archivo supera el límite de tamaño permitido",
			Error:   err.Error(),
		})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: fmt.Sprintf("Extensión de archivo no permitida ('%s')", ext),
			Error:   "UnsupportedFileExtension",
		})
		return
	}

	// Sanitizar nombre de archivo para prevenir Path Traversal
	filename := filepath.Base(header.Filename)
	filename = strings.ReplaceAll(filename, "..", "")
	if filename == "" || filename == "." {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Message: "Nombre de archivo inválido",
			Error:   "InvalidFilename",
		})
		return
	}

	targetURL := fmt.Sprintf("%s/uploads/%s/%s", h.cfg.ContentManagerURL, folder, filename)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPut, targetURL, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Message: "Error al preparar la solicitud de carga al gestor de contenidos",
			Error:   err.Error(),
		})
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{
			Message: "No se pudo conectar con el servidor de almacenamiento de contenidos",
			Error:   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, models.ErrorResponse{
			Message: fmt.Sprintf("Error devuelto por el gestor de contenidos (%d)", resp.StatusCode),
			Error:   string(body),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Archivo subido correctamente",
		"filename": filename,
		"folder":   folder,
	})
}
