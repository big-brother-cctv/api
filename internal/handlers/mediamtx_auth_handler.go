package handlers

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/models"
	"github.com/gin-gonic/gin"
)

// Usuarios convencionales para las credenciales de MediaMTX. El nombre de usuario es
// informativo: la autorización se decide únicamente por la contraseña/token.
const (
	MediaPublishUser = "publisher"
	MediaReadUser    = "reader"
	MediaAPIUser     = "api"
)

// MediaMTXAuthRequest es el cuerpo que MediaMTX envía con authMethod: http.
// https://mediamtx.org/docs/features/authentication
type MediaMTXAuthRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Token    string `json:"token"`
	IP       string `json:"ip"`
	Action   string `json:"action"`
	Path     string `json:"path"`
	Protocol string `json:"protocol"`
	ID       string `json:"id"`
	Query    string `json:"query"`
}

type MediaMTXAuthHandler struct {
	cfg *config.Config
}

func NewMediaMTXAuthHandler(cfg *config.Config) *MediaMTXAuthHandler {
	return &MediaMTXAuthHandler{cfg: cfg}
}

func secretEquals(provided, expected string) bool {
	return expected != "" && provided != "" &&
		subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// credential extrae el secreto presentado: password (Basic, user:pass en URL RTSP,
// ?user=&pass= en RTMP) o token (Bearer / ?token=). "Bearer user:pass" llega como token.
func (r *MediaMTXAuthRequest) credential() string {
	if r.Password != "" {
		return r.Password
	}
	if i := strings.LastIndex(r.Token, ":"); i >= 0 {
		return r.Token[i+1:]
	}
	return r.Token
}

// Authorize decide si MediaMTX debe permitir la acción. Exportado para tests.
func (h *MediaMTXAuthHandler) Authorize(req *MediaMTXAuthRequest) bool {
	secret := req.credential()
	internal := secretEquals(secret, h.cfg.InternalToken)

	switch req.Action {
	case "publish":
		// Solo nombres de path válidos como cámara (se auto-registran en la BD)
		if !models.IsValidCameraName(req.Path) {
			return false
		}
		return internal || secretEquals(secret, h.cfg.MediaPublishToken)
	case "read", "playback":
		return internal || secretEquals(secret, h.cfg.MediaReadToken)
	case "api", "metrics", "pprof":
		return internal
	default:
		return false
	}
}

// Auth es el endpoint authHTTPAddress de MediaMTX. 2xx = permitido; 401 = denegado
// (en RTSP, el 401 hace que el cliente reintente enviando credenciales).
func (h *MediaMTXAuthHandler) Auth(c *gin.Context) {
	var req MediaMTXAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if h.Authorize(&req) {
		c.Status(http.StatusOK)
		return
	}

	// Las primeras peticiones RTSP llegan sin credenciales por diseño: no son un intento fallido.
	if req.credential() != "" {
		log.Printf("[MEDIAMTX AUTH] Denegado: action=%s path=%q protocol=%s ip=%s user=%q",
			req.Action, req.Path, req.Protocol, req.IP, req.User)
	}
	c.Status(http.StatusUnauthorized)
}

// PublishCredentials devuelve las credenciales de publicación para configurar móviles/OBS
// desde el dashboard (solo admin/operator).
func (h *MediaMTXAuthHandler) PublishCredentials(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user":       MediaPublishUser,
		"password":   h.cfg.MediaPublishToken,
		"configured": h.cfg.MediaPublishToken != "",
	})
}
