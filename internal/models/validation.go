package models

import "regexp"

// cameraNameRegex es la única definición válida de nombre de cámara.
// El nombre se usa como path de MediaMTX, nombre de fichero y valor dentro de la
// configuración generada para Motion, así que NUNCA debe admitir espacios,
// saltos de línea, barras ni caracteres de shell.
var cameraNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{2,48}$`)

// IsValidCameraName valida un nombre de cámara para cualquier camino de alta
// (Create, Update, Heartbeat, auto-discovery de MediaMTX).
func IsValidCameraName(name string) bool {
	return cameraNameRegex.MatchString(name)
}
