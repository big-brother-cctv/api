package handlers

import (
	"testing"
	"time"
)

func TestAllowAlertUsesPerCameraCooldown(t *testing.T) {
	h := &AlertHandler{cooldowns: map[string]time.Time{}}
	t0 := time.Unix(1_000_000, 0)

	if !h.allowAlert("salon", t0, 5*time.Second) {
		t.Fatal("first alert must pass")
	}
	if h.allowAlert("salon", t0.Add(3*time.Second), 5*time.Second) {
		t.Fatal("alert inside 5s cooldown must be skipped")
	}
	// Cooldown de 5s: a los 6s debe pasar (antes la API forzaba 15s fijos)
	if !h.allowAlert("salon", t0.Add(6*time.Second), 5*time.Second) {
		t.Fatal("alert after 5s cooldown must pass")
	}
	// Cámaras independientes
	if !h.allowAlert("cocina", t0.Add(6*time.Second), 5*time.Second) {
		t.Fatal("other camera must not be affected")
	}
}
