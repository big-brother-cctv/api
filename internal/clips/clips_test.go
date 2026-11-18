package clips

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bigbrother/api/internal/models"
)

func TestCovers(t *testing.T) {
	base := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	start, end := base, base.Add(35*time.Second)
	seg := func(offset, dur float64) segment {
		return segment{Start: base.Add(time.Duration(offset * float64(time.Second))), Duration: dur}
	}
	cases := []struct {
		name string
		segs []segment
		want bool
	}{
		{"empty", nil, false},
		{"one segment covering all", []segment{seg(-5, 60)}, true},
		{"consecutive segments", []segment{seg(-3, 10), seg(7, 10), seg(17, 10), seg(27, 10)}, true},
		{"not yet reaching end", []segment{seg(-3, 10), seg(7, 10), seg(17, 10)}, false},
		{"stream started inside window (tolerance)", []segment{seg(0.5, 40)}, true},
		{"stream started late", []segment{seg(10, 40)}, false},
	}
	for _, tc := range cases {
		if got := Covers(tc.segs, start, end); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

type fakeUpdater struct {
	mu   sync.Mutex
	path map[uint]string
}

func (f *fakeUpdater) SetClipPath(id uint, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.path[id] = p
	return nil
}

// Simula el servidor de playback: /list cubre la ventana solo a partir de la 2ª consulta.
func TestCaptureWaitsForCoverageAndStoresClip(t *testing.T) {
	alertAt := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	listCalls := 0
	var gotGet *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "api" || p != "internal-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/list":
			listCalls++
			dur := 20.0
			if listCalls >= 2 {
				dur = 60.0
			}
			_ = json.NewEncoder(w).Encode([]segment{{Start: alertAt.Add(-40 * time.Second), Duration: dur}})
		case "/get":
			gotGet = r
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("fake-mp4-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	up := &fakeUpdater{path: map[uint]string{}}
	svc := New(Config{PlaybackURL: srv.URL, InternalToken: "internal-token", Dir: dir, Pre: 30 * time.Second, Post: 5 * time.Second}, up)
	clock := alertAt
	svc.now = func() time.Time { return clock }
	svc.sleep = func(_ context.Context, d time.Duration) bool { clock = clock.Add(d); return true }

	svc.Capture(context.Background(), models.MotionAlert{ID: 7, CameraName: "salon", DetectedAt: alertAt})

	if listCalls < 2 {
		t.Fatalf("expected to poll /list until covered, got %d calls", listCalls)
	}
	if gotGet == nil {
		t.Fatal("expected a /get request")
	}
	q := gotGet.URL.Query()
	if q.Get("path") != "salon" || q.Get("format") != "mp4" || q.Get("duration") != "35.000" {
		t.Errorf("unexpected /get query: %v", q)
	}
	if st, _ := time.Parse(time.RFC3339Nano, q.Get("start")); !st.Equal(alertAt.Add(-30 * time.Second)) {
		t.Errorf("unexpected start %s", q.Get("start"))
	}
	if up.path[7] != "/api/alerts/snapshots/clip_7_salon.mp4" {
		t.Fatalf("clip path not stored: %v", up.path)
	}
	data, err := os.ReadFile(filepath.Join(dir, "clip_7_salon.mp4"))
	if err != nil || string(data) != "fake-mp4-bytes" {
		t.Fatalf("clip file not written correctly: %q %v", data, err)
	}
}

func TestCaptureGivesUpWithoutRecording(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // nunca hubo grabación (p. ej. cámara IP sin record)
	}))
	defer srv.Close()

	alertAt := time.Now()
	up := &fakeUpdater{path: map[uint]string{}}
	svc := New(Config{PlaybackURL: srv.URL, InternalToken: "t", Dir: t.TempDir(), Pre: 30 * time.Second, Post: 5 * time.Second}, up)
	clock := alertAt
	svc.now = func() time.Time { return clock }
	svc.sleep = func(_ context.Context, d time.Duration) bool { clock = clock.Add(d); return true }

	svc.Capture(context.Background(), models.MotionAlert{ID: 1, CameraName: "salon", DetectedAt: alertAt})
	if len(up.path) != 0 {
		t.Fatal("no clip must be stored when there is no recording")
	}
}
