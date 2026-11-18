package auth

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignedSnapshotURL(t *testing.T) {
	svc := NewJWTService("a-very-long-random-jwt-secret-value-0123456789")
	now := time.Now()
	u := svc.SignSnapshotURL("/api/alerts/snapshots/alert_1_salon.jpg", now)
	if !strings.HasPrefix(u, "/api/alerts/snapshots/alert_1_salon.jpg?") {
		t.Fatalf("unexpected url %s", u)
	}
	parsed, _ := url.Parse(u)
	q := parsed.Query()

	if !svc.VerifySnapshotSignature("alert_1_salon.jpg", q.Get("exp"), q.Get("sig"), now) {
		t.Fatal("valid signature rejected")
	}
	if svc.VerifySnapshotSignature("alert_2_salon.jpg", q.Get("exp"), q.Get("sig"), now) {
		t.Fatal("signature must be bound to the file")
	}
	if svc.VerifySnapshotSignature("alert_1_salon.jpg", q.Get("exp"), q.Get("sig"), now.Add(SnapshotURLTTL+time.Minute)) {
		t.Fatal("expired signature accepted")
	}
	other := NewJWTService("another-very-long-random-jwt-secret-value-987")
	if other.VerifySnapshotSignature("alert_1_salon.jpg", q.Get("exp"), q.Get("sig"), now) {
		t.Fatal("signature from another key accepted")
	}
}
