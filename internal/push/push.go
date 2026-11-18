// Package push envía notificaciones Web Push (VAPID, RFC 8291) a navegadores y PWAs,
// para que las alertas lleguen con el móvil bloqueado o la app cerrada (ADR-005).
package push

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/bigbrother/api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	settingVAPIDPublic  = "vapid_public_key"
	settingVAPIDPrivate = "vapid_private_key"

	maxEndpointLen = 2048
	maxKeyLen      = 256
	maxConcurrent  = 4
	pushTTLSeconds = 3600
)

// Servicios push reales de los navegadores. Solo se envían peticiones a estos dominios:
// el endpoint lo aporta el cliente y, sin lista blanca, permitiría SSRF hacia la red interna.
var allowedPushHostSuffixes = []string{
	".googleapis.com",     // Chrome / Android (FCM)
	".mozilla.com",        // Firefox
	".mozaws.net",         // Firefox (autopush)
	".push.apple.com",     // Safari / iOS PWA
	".notify.windows.com", // Edge (WNS)
}

var ErrInvalidSubscription = errors.New("suscripción push no válida")

// SubscriptionInput es el JSON de PushSubscription.toJSON() del navegador.
type SubscriptionInput struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Payload es lo que recibe el service worker (telescreen/public/sw.js).
type Payload struct {
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Tag   string         `json:"tag,omitempty"`
	Image string         `json:"image,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}

type Service struct {
	db         *gorm.DB
	publicKey  string
	privateKey string
	subject    string
	client     *http.Client
	sem        chan struct{}
	wg         sync.WaitGroup
}

// NewService carga las claves VAPID de la configuración o, si no se dan, de la BD;
// si tampoco existen allí, las genera una vez y las guarda (así no cambian entre reinicios
// y las suscripciones existentes siguen siendo válidas).
func NewService(db *gorm.DB, publicKey, privateKey, subject string) (*Service, error) {
	if subject == "" {
		subject = "https://github.com/big-brother-cctv"
	}
	s := &Service{
		db:      db,
		subject: subject,
		client:  &http.Client{Timeout: 10 * time.Second},
		sem:     make(chan struct{}, maxConcurrent),
	}

	if publicKey != "" && privateKey != "" {
		s.publicKey, s.privateKey = publicKey, privateKey
		return s, nil
	}

	pub, priv, err := loadKeys(db)
	if err != nil {
		return nil, err
	}
	if pub == "" || priv == "" {
		priv, pub, err = webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, fmt.Errorf("generando claves VAPID: %w", err)
		}
		if err := saveKeys(db, pub, priv); err != nil {
			return nil, err
		}
		log.Printf("[PUSH] Claves VAPID generadas y guardadas en la base de datos")
	}
	s.publicKey, s.privateKey = pub, priv
	return s, nil
}

func loadKeys(db *gorm.DB) (string, string, error) {
	var settings []models.AppSetting
	if err := db.Where("key IN ?", []string{settingVAPIDPublic, settingVAPIDPrivate}).Find(&settings).Error; err != nil {
		return "", "", err
	}
	var pub, priv string
	for _, st := range settings {
		switch st.Key {
		case settingVAPIDPublic:
			pub = st.Value
		case settingVAPIDPrivate:
			priv = st.Value
		}
	}
	return pub, priv, nil
}

func saveKeys(db *gorm.DB, pub, priv string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for k, v := range map[string]string{settingVAPIDPublic: pub, settingVAPIDPrivate: priv} {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&models.AppSetting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// PublicKey es la applicationServerKey que necesita pushManager.subscribe().
func (s *Service) PublicKey() string { return s.publicKey }

// ValidateSubscription comprueba formato y que el endpoint pertenezca a un servicio push conocido.
func ValidateSubscription(in SubscriptionInput) error {
	if len(in.Endpoint) == 0 || len(in.Endpoint) > maxEndpointLen {
		return ErrInvalidSubscription
	}
	u, err := url.Parse(in.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ErrInvalidSubscription
	}
	host := strings.ToLower(u.Hostname())
	allowed := false
	for _, suffix := range allowedPushHostSuffixes {
		if strings.HasSuffix(host, suffix) || host == strings.TrimPrefix(suffix, ".") {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrInvalidSubscription
	}
	if in.Keys.P256dh == "" || in.Keys.Auth == "" || len(in.Keys.P256dh) > maxKeyLen || len(in.Keys.Auth) > maxKeyLen {
		return ErrInvalidSubscription
	}
	return nil
}

// Subscribe registra o actualiza (por endpoint) la suscripción de un usuario.
func (s *Service) Subscribe(username string, in SubscriptionInput, userAgent string) error {
	if err := ValidateSubscription(in); err != nil {
		return err
	}
	if len(userAgent) > 255 {
		userAgent = userAgent[:255]
	}
	sub := models.PushSubscription{
		Username:  username,
		Endpoint:  in.Endpoint,
		P256dh:    in.Keys.P256dh,
		Auth:      in.Keys.Auth,
		UserAgent: userAgent,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{"username", "p256dh", "auth", "user_agent"}),
	}).Create(&sub).Error
}

// Unsubscribe borra la suscripción del endpoint si pertenece al usuario.
func (s *Service) Unsubscribe(username, endpoint string) error {
	return s.db.Where("endpoint = ? AND LOWER(username) = LOWER(?)", endpoint, username).
		Delete(&models.PushSubscription{}).Error
}

// NotifyAlert envía la alerta a todos los usuarios activos con suscripción. No bloquea.
func (s *Service) NotifyAlert(alert models.MotionAlert, imageURL string) {
	label := alert.Label
	if label == "" {
		label = "motion"
	}
	payload := Payload{
		Title: fmt.Sprintf("🚨 %s", strings.ToUpper(alert.CameraName)),
		Body:  fmt.Sprintf("Detectado: %s · %s", label, alert.DetectedAt.Local().Format("15:04:05")),
		Tag:   fmt.Sprintf("bb-alert-%d", alert.ID),
		Image: imageURL,
		Data:  map[string]any{"url": "/?view=alerts", "id": alert.ID},
	}

	var subs []models.PushSubscription
	err := s.db.Where("LOWER(username) IN (?)",
		s.db.Model(&models.User{}).Select("LOWER(username)").Where("disabled = ?", false),
	).Find(&subs).Error
	if err != nil {
		log.Printf("[PUSH] Error cargando suscripciones: %v", err)
		return
	}
	s.sendAll(subs, payload)
}

// NotifyUser envía una notificación a las suscripciones de un usuario. Devuelve cuántas había.
func (s *Service) NotifyUser(username string, payload Payload) (int, error) {
	var subs []models.PushSubscription
	if err := s.db.Where("LOWER(username) = LOWER(?)", username).Find(&subs).Error; err != nil {
		return 0, err
	}
	s.sendAll(subs, payload)
	return len(subs), nil
}

func (s *Service) sendAll(subs []models.PushSubscription, payload Payload) {
	if len(subs) == 0 {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[PUSH] Error serializando payload: %v", err)
		return
	}
	for _, sub := range subs {
		sub := sub
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.sem <- struct{}{}
			defer func() { <-s.sem }()
			s.send(sub, body)
		}()
	}
}

func (s *Service) send(sub models.PushSubscription, body []byte) {
	resp, err := webpush.SendNotification(body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      s.client,
		Subscriber:      s.subject,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		TTL:             pushTTLSeconds,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		log.Printf("[PUSH] Error enviando a suscripción %d (%s): %v", sub.ID, hostOf(sub.Endpoint), err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		// El navegador revocó la suscripción: limpiarla
		s.db.Delete(&models.PushSubscription{}, sub.ID)
		log.Printf("[PUSH] Suscripción %d caducada (%d), eliminada", sub.ID, resp.StatusCode)
	case resp.StatusCode >= 400:
		log.Printf("[PUSH] El servicio push (%s) respondió %d a la suscripción %d", hostOf(sub.Endpoint), resp.StatusCode, sub.ID)
	default:
		now := time.Now()
		s.db.Model(&models.PushSubscription{}).Where("id = ?", sub.ID).Update("last_success_at", &now)
	}
}

// Wait espera a que terminen los envíos en curso (tests y apagado).
func (s *Service) Wait() { s.wg.Wait() }

func hostOf(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return u.Hostname()
	}
	return "?"
}
