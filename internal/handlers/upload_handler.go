package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/bigbrother/api/internal/config"
	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	cfg *config.Config
}

func NewUploadHandler(cfg *config.Config) *UploadHandler {
	return &UploadHandler{cfg: cfg}
}

func (h *UploadHandler) UploadImage(c *gin.Context) {
	h.proxyUpload(c, "images")
}

func (h *UploadHandler) UploadVideo(c *gin.Context) {
	h.proxyUpload(c, "videos")
}

func (h *UploadHandler) proxyUpload(c *gin.Context, folder string) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file provided: "+err.Error())
		return
	}
	defer file.Close()

	filename := filepath.Base(header.Filename)
	targetURL := fmt.Sprintf("%s/uploads/%s/%s", h.cfg.ContentManagerURL, folder, filename)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPut, targetURL, file)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error creating request: "+err.Error())
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
		c.String(http.StatusInternalServerError, "Error uploading file: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		c.String(resp.StatusCode, fmt.Sprintf("Error from content manager (%d): %s", resp.StatusCode, string(body)))
		return
	}

	c.String(http.StatusOK, "File uploaded successfully: "+filename)
}
