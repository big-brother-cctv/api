package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"time"
)

// SnapshotURLTTL es la validez de las URLs firmadas de snapshots incluidas en notificaciones push.
const SnapshotURLTTL = 24 * time.Hour

func (s *JWTService) snapshotSignature(filename string, exp int64) string {
	mac := hmac.New(sha256.New, s.secretKey)
	fmt.Fprintf(mac, "snapshot:%s:%d", filename, exp)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignSnapshotURL devuelve la URL relativa del snapshot con firma y caducidad. Sirve para
// imágenes que el navegador o el sistema operativo descargan sin la cookie de sesión
// (p. ej. la imagen de una notificación push).
func (s *JWTService) SignSnapshotURL(snapshotPath string, now time.Time) string {
	filename := path.Base(snapshotPath)
	exp := now.Add(SnapshotURLTTL).Unix()
	q := url.Values{}
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", s.snapshotSignature(filename, exp))
	return "/api/alerts/snapshots/" + url.PathEscape(filename) + "?" + q.Encode()
}

// VerifySnapshotSignature comprueba firma y caducidad para un nombre de fichero.
func (s *JWTService) VerifySnapshotSignature(filename, expStr, sig string, now time.Time) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || now.Unix() > exp {
		return false
	}
	expected := s.snapshotSignature(path.Base(filename), exp)
	return hmac.Equal([]byte(expected), []byte(sig))
}
