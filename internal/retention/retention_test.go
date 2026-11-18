package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/bigbrother/api/internal/models"
)

type fakeStore struct {
	alerts map[uint]models.MotionAlert
}

func (f *fakeStore) OldestAlerts(before time.Time, limit int) ([]models.MotionAlert, error) {
	var out []models.MotionAlert
	for _, a := range f.alerts {
		if before.IsZero() || a.DetectedAt.Before(before) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DetectedAt.Before(out[j].DetectedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) DeleteAlerts(ids []uint) error {
	for _, id := range ids {
		delete(f.alerts, id)
	}
	return nil
}

func (f *fakeStore) SnapshotFiles() (map[string]bool, error) {
	m := map[string]bool{}
	for _, a := range f.alerts {
		for _, p := range []string{a.SnapshotPath, a.ClipPath} {
			if p != "" {
				m[filepath.Base(p)] = true
			}
		}
	}
	return m, nil
}

// setup crea n alertas, una por día hacia atrás desde now, cada una con un snapshot de size bytes.
func setup(t *testing.T, n int, size int, now time.Time) (*fakeStore, string) {
	t.Helper()
	dir := t.TempDir()
	st := &fakeStore{alerts: map[uint]models.MotionAlert{}}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("alert_%d.jpg", i)
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		st.alerts[uint(i+1)] = models.MotionAlert{
			ID:           uint(i + 1),
			SnapshotPath: "/api/alerts/snapshots/" + name,
			DetectedAt:   now.Add(-time.Duration(i) * 24 * time.Hour),
		}
	}
	return st, dir
}

func fileCount(t *testing.T, dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestExpiresOldAlertsAndFiles(t *testing.T) {
	now := time.Now()
	st, dir := setup(t, 10, 100, now)
	svc := New(st, dir, Policy{MaxAge: 5*24*time.Hour - time.Minute})
	svc.now = func() time.Time { return now }

	res, err := svc.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	// Se conservan los días 0..4 (5 alertas)
	if res.ExpiredAlerts != 5 || len(st.alerts) != 5 || fileCount(t, dir) != 5 {
		t.Fatalf("expected 5 expired / 5 kept, got res=%+v alerts=%d files=%d", res, len(st.alerts), fileCount(t, dir))
	}
	if res.BytesFreed != 500 {
		t.Fatalf("expected 500 bytes freed, got %d", res.BytesFreed)
	}
}

func TestEvictsOldestWhenOverStorageLimit(t *testing.T) {
	now := time.Now()
	st, dir := setup(t, 10, 100, now) // 1000 bytes
	svc := New(st, dir, Policy{MaxBytes: 650})
	svc.now = func() time.Time { return now }

	res, err := svc.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	// Hay que bajar de 1000 a <=650: se borran las 4 más antiguas
	if res.EvictedAlerts != 4 || fileCount(t, dir) != 6 {
		t.Fatalf("expected 4 evicted / 6 files, got res=%+v files=%d", res, fileCount(t, dir))
	}
	for _, a := range st.alerts {
		if now.Sub(a.DetectedAt) > 6*24*time.Hour {
			t.Fatalf("an old alert survived eviction: %v", a.DetectedAt)
		}
	}
}

func TestRemovesOnlyOldOrphans(t *testing.T) {
	now := time.Now()
	st, dir := setup(t, 2, 10, now)
	oldOrphan := filepath.Join(dir, "orphan_old.jpg")
	newOrphan := filepath.Join(dir, "orphan_new.jpg")
	_ = os.WriteFile(oldOrphan, []byte("x"), 0o644)
	_ = os.WriteFile(newOrphan, []byte("x"), 0o644)
	old := now.Add(-2 * time.Hour)
	_ = os.Chtimes(oldOrphan, old, old)

	svc := New(st, dir, Policy{MaxAge: 365 * 24 * time.Hour})
	svc.now = func() time.Time { return now }
	res, err := svc.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.OrphanFiles != 1 {
		t.Fatalf("expected 1 orphan removed, got %d", res.OrphanFiles)
	}
	if _, err := os.Stat(newOrphan); err != nil {
		t.Fatal("recent orphan (possible in-flight write) must be kept")
	}
	if len(st.alerts) != 2 {
		t.Fatal("referenced alerts must be kept")
	}
}

func TestDisabledPolicyDoesNothing(t *testing.T) {
	now := time.Now()
	st, dir := setup(t, 3, 10, now)
	svc := New(st, dir, Policy{})
	res, err := svc.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpiredAlerts+res.EvictedAlerts != 0 || len(st.alerts) != 3 {
		t.Fatalf("disabled policy must not delete alerts: %+v", res)
	}
}

func TestClipsAreDeletedWithTheirAlertAndNotTreatedAsOrphans(t *testing.T) {
	now := time.Now()
	st, dir := setup(t, 2, 100, now)
	// Alerta 1 (reciente) y 2 (de hace un día) con clip; ficheros antiguos para que cuenten como posibles huérfanos
	old := now.Add(-3 * time.Hour)
	for _, id := range []uint{1, 2} {
		a := st.alerts[id]
		name := fmt.Sprintf("clip_%d.mp4", id)
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, make([]byte, 1000), 0o644)
		_ = os.Chtimes(p, old, old)
		a.ClipPath = "/api/alerts/snapshots/" + name
		st.alerts[id] = a
	}
	// Temporal de escritura reciente: no tocar
	tmp := filepath.Join(dir, ".clip_9.mp4.tmp")
	_ = os.WriteFile(tmp, []byte("x"), 0o644)

	svc := New(st, dir, Policy{MaxAge: 12 * time.Hour})
	svc.now = func() time.Time { return now }
	res, err := svc.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpiredAlerts != 1 || res.OrphanFiles != 0 {
		t.Fatalf("expected 1 expired alert and no orphans, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "clip_1.mp4")); err != nil {
		t.Fatal("clip of a kept alert must not be deleted as orphan")
	}
	if _, err := os.Stat(filepath.Join(dir, "clip_2.mp4")); !os.IsNotExist(err) {
		t.Fatal("clip of an expired alert must be deleted")
	}
	if res.BytesFreed != 1100 {
		t.Fatalf("expected snapshot+clip bytes freed (1100), got %d", res.BytesFreed)
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatal("recent temp file must be kept")
	}
}
