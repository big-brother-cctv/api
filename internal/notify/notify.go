// Package notify decide qué alertas se notifican (push, toast y alarma) según el modo
// "en casa"/"fuera" y las clases de detección elegidas para cada modo. Las alertas se
// guardan siempre; esta política solo controla si se avisa.
package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Mode string

const (
	ModeHome Mode = "home"
	ModeAway Mode = "away"

	// ClassMotion: movimiento sin clasificar (IA desactivada o sin modelo).
	ClassMotion = "motion"
	// ClassOther: cualquier clase de la IA que no esté en la lista conocida.
	ClassOther = "other"

	settingKey = "notification_settings"
)

// Classes son las clases configurables, en el orden en que se muestran en el dashboard.
// Coinciden con las que motion-detector puede etiquetar (YOLOv8/COCO) más motion/other.
var Classes = []string{"person", "cat", "dog", "bird", "car", "truck", "bus", "motorcycle", "bicycle", ClassMotion, ClassOther}

var knownClass = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range Classes {
		m[c] = true
	}
	return m
}()

var ErrInvalidSettings = errors.New("configuración de notificaciones no válida")

type Settings struct {
	Mode      Mode      `json:"mode"`
	Home      []string  `json:"home"`
	Away      []string  `json:"away"`
	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`
}

// Defaults: fuera avisa de todo; en casa no avisa (las alertas se siguen guardando).
func Defaults() Settings {
	return Settings{Mode: ModeAway, Home: []string{}, Away: append([]string{}, Classes...)}
}

var labelRe = regexp.MustCompile(`^\s*([a-zA-Z _-]+?)\s*(?:\([0-9.]+%?\))?\s*$`)

// ClassOf normaliza una etiqueta de alerta ("cat (87%)", "person", "motion", "") a su clase.
func ClassOf(label string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	if l == "" || l == "motion" || l == "movimiento" {
		return ClassMotion
	}
	if m := labelRe.FindStringSubmatch(l); m != nil {
		l = strings.TrimSpace(m[1])
	}
	if knownClass[l] && l != ClassOther {
		return l
	}
	return ClassOther
}

// ShouldNotify indica si una alerta con esa etiqueta debe avisar en el modo actual.
func (s Settings) ShouldNotify(label string) bool {
	list := s.Away
	if s.Mode == ModeHome {
		list = s.Home
	}
	class := ClassOf(label)
	for _, c := range list {
		if c == class {
			return true
		}
	}
	return false
}

func normalizeClasses(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		if !knownClass[c] {
			return nil, fmt.Errorf("%w: clase desconocida %q", ErrInvalidSettings, c)
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out, nil
}

// Validate normaliza y valida la configuración.
func (s *Settings) Validate() error {
	if s.Mode != ModeHome && s.Mode != ModeAway {
		return fmt.Errorf("%w: modo %q", ErrInvalidSettings, s.Mode)
	}
	var err error
	if s.Home, err = normalizeClasses(s.Home); err != nil {
		return err
	}
	if s.Away, err = normalizeClasses(s.Away); err != nil {
		return err
	}
	return nil
}

// Load lee la configuración; si no existe o está corrupta devuelve los valores por defecto.
func Load(db *gorm.DB) Settings {
	var st models.AppSetting
	if err := db.Where("key = ?", settingKey).First(&st).Error; err != nil {
		return Defaults()
	}
	s := Defaults()
	if err := json.Unmarshal([]byte(st.Value), &s); err != nil || s.Validate() != nil {
		return Defaults()
	}
	return s
}

// Save valida y guarda la configuración.
func Save(db *gorm.DB, s Settings, by string, now time.Time) (Settings, error) {
	if err := s.Validate(); err != nil {
		return s, err
	}
	s.UpdatedAt, s.UpdatedBy = now, by
	raw, err := json.Marshal(s)
	if err != nil {
		return s, err
	}
	err = db.Clauses(clause.OnConflict{UpdateAll: true}).
		Create(&models.AppSetting{Key: settingKey, Value: string(raw)}).Error
	return s, err
}
