package mediamtx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/models"
	"gorm.io/gorm"
)

type Service struct {
	apiURL   string
	rtspHost string
	client   *http.Client
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		apiURL:   strings.TrimRight(cfg.MediaMTXAPIURL, "/"),
		rtspHost: cfg.MediaMTXRTSPHost,
		client: &http.Client{
			Timeout: 4 * time.Second,
		},
	}
}

type PathItem struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Readers int    `json:"readers"`
}

type PathsListResponse struct {
	ItemCount int        `json:"itemCount"`
	PageCount int        `json:"pageCount"`
	Items     []PathItem `json:"items"`
}

// GetActivePaths consulta la API de MediaMTX para saber qué streams están activos
func (s *Service) GetActivePaths() (map[string]bool, error) {
	url := fmt.Sprintf("%s/v3/paths/list", s.apiURL)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mediamtx paths list returned status %d", resp.StatusCode)
	}

	var data PathsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	activeMap := make(map[string]bool)
	for _, item := range data.Items {
		activeMap[item.Name] = item.Ready
	}
	return activeMap, nil
}

// ConfigurePullPath registra o actualiza una cámara IP remota en MediaMTX para que éste haga PULL autónomo
func (s *Service) ConfigurePullPath(cameraName, sourceURL string) error {
	if sourceURL == "" {
		return nil
	}
	urlAdd := fmt.Sprintf("%s/v3/config/paths/add/%s", s.apiURL, cameraName)
	payload := map[string]interface{}{
		"source":         sourceURL,
		"sourceOnDemand": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, urlAdd, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Si ya existía, intentamos actualizar con PATCH
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusConflict {
		urlPatch := fmt.Sprintf("%s/v3/config/paths/patch/%s", s.apiURL, cameraName)
		patchReq, err := http.NewRequest(http.MethodPatch, urlPatch, bytes.NewBuffer(body))
		if err != nil {
			return err
		}
		patchReq.Header.Set("Content-Type", "application/json")
		patchResp, err := s.client.Do(patchReq)
		if err != nil {
			return err
		}
		defer patchResp.Body.Close()
	}

	return nil
}

// RemovePath elimina la ruta de MediaMTX
func (s *Service) RemovePath(cameraName string) error {
	urlDel := fmt.Sprintf("%s/v3/config/paths/delete/%s", s.apiURL, cameraName)
	req, err := http.NewRequest(http.MethodDelete, urlDel, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// StartSyncLoop sincroniza cada pocos segundos el estado de los streams y auto-descubre cámaras entrantes (PUSH)
func (s *Service) StartSyncLoop(db *gorm.DB) {
	ticker := time.NewTicker(4 * time.Second)
	go func() {
		for range ticker.C {
			activePaths, err := s.GetActivePaths()
			if err != nil {
				// MediaMTX puede estar arrancando o no accesible aún
				continue
			}

			var cameras []models.Camera
			if err := db.Find(&cameras).Error; err != nil {
				continue
			}

			existingMap := make(map[string]*models.Camera)
			now := time.Now()

			for i := range cameras {
				cam := &cameras[i]
				existingMap[strings.ToLower(cam.Name)] = cam

				isStreaming, existsInMtx := activePaths[cam.Name]
				if !existsInMtx {
					isStreaming, existsInMtx = activePaths[strings.ToLower(cam.Name)]
				}

				newStatus := cam.Status
				if isStreaming {
					newStatus = "online"
				} else {
					// Si no está en MediaMTX o no está ready:
					if cam.LastSeen == nil || now.Sub(*cam.LastSeen) > 15*time.Second {
						newStatus = "offline"
					} else {
						newStatus = "standby"
					}
				}

				if newStatus != cam.Status {
					db.Model(&models.Camera{}).Where("id = ?", cam.ID).Update("status", newStatus)
				}
			}

			// AUTO-DESCUBRIMIENTO: Si entra un stream a MediaMTX no registrado en la BD, lo auto-registramos
			for pathName, isReady := range activePaths {
				lowerName := strings.ToLower(pathName)
				if _, exists := existingMap[lowerName]; !exists && isReady {
					log.Printf("[INGSOC AUTO-DISCOVERY] Nuevo stream detectado en MediaMTX: '%s'. Auto-registrando en Telescreen...", pathName)
					newCam := models.Camera{
						Name:           pathName,
						SourceType:     "rtsp",
						ConnectionMode: "push",
						Resolution:     "AUTO-DETECT",
						FPS:            "AUTO",
						Codec:          "h264",
						Preset:         "ultrafast",
						Tune:           "zerolatency",
						Status:         "online",
						LastSeen:       &now,
						Device:         "remote-push",
					}
					if err := db.Create(&newCam).Error; err != nil {
						log.Printf("Error al auto-registrar cámara '%s': %v", pathName, err)
					}
				}
			}
		}
	}()
}
