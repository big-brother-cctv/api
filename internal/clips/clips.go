// Package clips extrae un clip de vídeo por alerta desde el servidor de playback de MediaMTX.
//
// MediaMTX graba un búfer rotativo corto (en RAM, ver chart mediamtx: recording.*). Cuando
// llega una alerta, se espera a que la grabación cubra la ventana [alerta-pre, alerta+post]
// (consultando /list) y se descarga ese tramo con /get?format=mp4, sin recodificar.
// La foto de Motion se guarda al FINAL del evento, por eso la ventana mira sobre todo atrás.
package clips

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/models"
	"gorm.io/gorm"
)

const (
	maxClipBytes = 200 << 20 // 200 MB: tope defensivo por clip
	// Tiempo máximo para que la grabación cubra la ventana del clip antes de intentarlo igualmente.
	coverageTimeout = 90 * time.Second
	pollInterval    = 5 * time.Second
	// Margen tras el final de la ventana antes de la primera consulta (parts de 1 s + escritura).
	settleDelay = 3 * time.Second
)

type Config struct {
	PlaybackURL   string
	InternalToken string
	Dir           string
	Pre           time.Duration
	Post          time.Duration
}

// Updater persiste la ruta del clip en la alerta. Abstracción para tests.
type Updater interface {
	SetClipPath(alertID uint, clipPath string) error
}

type Service struct {
	cfg     Config
	updater Updater
	client  *http.Client
	now     func() time.Time
	sleep   func(context.Context, time.Duration) bool
}

func New(cfg Config, updater Updater) *Service {
	return &Service{
		cfg:     cfg,
		updater: updater,
		client:  &http.Client{Timeout: 60 * time.Second},
		now:     time.Now,
		sleep: func(ctx context.Context, d time.Duration) bool {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return false
			case <-t.C:
				return true
			}
		},
	}
}

// Window devuelve el tramo de vídeo de una alerta.
func (s *Service) Window(alertAt time.Time) (start, end time.Time) {
	return alertAt.Add(-s.cfg.Pre), alertAt.Add(s.cfg.Post)
}

// Capture extrae el clip de la alerta (bloquea hasta ~2 min; llamar en una goroutine).
func (s *Service) Capture(ctx context.Context, alert models.MotionAlert) {
	start, end := s.Window(alert.DetectedAt)

	if wait := end.Add(settleDelay).Sub(s.now()); wait > 0 && !s.sleep(ctx, wait) {
		return
	}

	deadline := s.now().Add(coverageTimeout)
	for {
		covered, err := s.covered(ctx, alert.CameraName, start, end)
		if err != nil {
			log.Printf("[CLIPS] Alerta %d (%s): no se pudo consultar la grabación: %v", alert.ID, alert.CameraName, err)
		}
		if covered || s.now().After(deadline) {
			break
		}
		if !s.sleep(ctx, pollInterval) {
			return
		}
	}

	filename, err := s.download(ctx, alert, start, end)
	if err != nil {
		log.Printf("[CLIPS] Alerta %d (%s): sin clip: %v", alert.ID, alert.CameraName, err)
		return
	}
	clipPath := "/api/alerts/snapshots/" + filename
	if err := s.updater.SetClipPath(alert.ID, clipPath); err != nil {
		_ = os.Remove(filepath.Join(s.cfg.Dir, filename))
		log.Printf("[CLIPS] Alerta %d: error guardando la ruta del clip: %v", alert.ID, err)
		return
	}
	log.Printf("[CLIPS] Alerta %d (%s): clip guardado (%s)", alert.ID, alert.CameraName, filename)
}

type segment struct {
	Start    time.Time `json:"start"`
	Duration float64   `json:"duration"`
}

// covered indica si las grabaciones existentes llegan hasta el final de la ventana.
func (s *Service) covered(ctx context.Context, path string, start, end time.Time) (bool, error) {
	q := url.Values{}
	q.Set("path", path)
	q.Set("start", start.Format(time.RFC3339Nano))
	q.Set("end", end.Format(time.RFC3339Nano))
	body, status, err := s.get(ctx, "/list?"+q.Encode(), 1<<20)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil // aún no hay grabación para ese tramo
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("/list respondió %d", status)
	}
	var segs []segment
	if err := json.Unmarshal(body, &segs); err != nil {
		return false, fmt.Errorf("/list: %w", err)
	}
	return Covers(segs, start, end), nil
}

// Covers indica si los segmentos llegan desde start (o antes) hasta end. Exportado para tests.
func Covers(segs []segment, start, end time.Time) bool {
	var first, last time.Time
	for i, sg := range segs {
		segEnd := sg.Start.Add(time.Duration(sg.Duration * float64(time.Second)))
		if i == 0 || sg.Start.Before(first) {
			first = sg.Start
		}
		if segEnd.After(last) {
			last = segEnd
		}
	}
	if len(segs) == 0 {
		return false
	}
	// Tolerancia de 1 s en el inicio: el stream pudo empezar justo dentro de la ventana
	return !first.After(start.Add(time.Second)) && !last.Before(end)
}

func (s *Service) download(ctx context.Context, alert models.MotionAlert, start, end time.Time) (string, error) {
	q := url.Values{}
	q.Set("path", alert.CameraName)
	q.Set("start", start.Format(time.RFC3339Nano))
	q.Set("duration", fmt.Sprintf("%.3f", end.Sub(start).Seconds()))
	q.Set("format", "mp4") // mp4 clásico: mejor compatibilidad (iOS Safari) que fmp4
	body, status, err := s.get(ctx, "/get?"+q.Encode(), maxClipBytes)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("/get respondió %d (¿la cámara no estaba grabando?)", status)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("clip vacío")
	}

	filename := fmt.Sprintf("clip_%d_%s.mp4", alert.ID, safeName(alert.CameraName))
	tmp := filepath.Join(s.cfg.Dir, "."+filename+".tmp")
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(s.cfg.Dir, filename)); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return filename, nil
}

func (s *Service) get(ctx context.Context, pathAndQuery string, limit int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.cfg.PlaybackURL, "/")+pathAndQuery, nil)
	if err != nil {
		return nil, 0, err
	}
	// Acción "playback": la API se autentica con su token interno (ver /internal/mediamtx/auth)
	req.SetBasicAuth("api", s.cfg.InternalToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if int64(len(body)) > limit {
		return nil, resp.StatusCode, fmt.Errorf("respuesta mayor de %d bytes", limit)
	}
	return body, resp.StatusCode, nil
}

func safeName(name string) string {
	if models.IsValidCameraName(name) {
		return name
	}
	return "unknown"
}

// GormUpdater implementa Updater sobre la tabla alerts.
type GormUpdater struct{ DB *gorm.DB }

func (g GormUpdater) SetClipPath(alertID uint, clipPath string) error {
	return g.DB.Model(&models.MotionAlert{}).Where("id = ?", alertID).Update("clip_path", clipPath).Error
}
