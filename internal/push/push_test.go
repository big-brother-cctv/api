package push

import "testing"

func sub(endpoint string) SubscriptionInput {
	var in SubscriptionInput
	in.Endpoint = endpoint
	in.Keys.P256dh = "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM"
	in.Keys.Auth = "tBHItJI5svbpez7KI4CCXg"
	return in
}

func TestValidateSubscriptionAllowsRealPushServices(t *testing.T) {
	for _, ep := range []string{
		"https://fcm.googleapis.com/fcm/send/abc:APA91b",
		"https://updates.push.services.mozilla.com/wpush/v2/gAAAAA",
		"https://web.push.apple.com/QGuQyavXutnMH",
		"https://wns2-db5p.notify.windows.com/w/?token=abc",
	} {
		if err := ValidateSubscription(sub(ep)); err != nil {
			t.Errorf("%s should be allowed: %v", ep, err)
		}
	}
}

func TestValidateSubscriptionRejectsSSRFAndGarbage(t *testing.T) {
	for _, ep := range []string{
		"",
		"http://fcm.googleapis.com/fcm/send/abc", // sin TLS
		"https://api:8080/internal/mediamtx/auth",         // red interna
		"https://169.254.169.254/latest/meta-data",        // metadata
		"https://evil.example.com/push",                   // dominio no permitido
		"https://fcm.googleapis.com.evil.com/x",           // sufijo engañoso
		"https://user:pass@fcm.googleapis.com/fcm/send/x", // credenciales en URL
		"ftp://fcm.googleapis.com/x",
	} {
		if err := ValidateSubscription(sub(ep)); err == nil {
			t.Errorf("%q should be rejected", ep)
		}
	}

	missingKeys := sub("https://fcm.googleapis.com/fcm/send/abc")
	missingKeys.Keys.Auth = ""
	if err := ValidateSubscription(missingKeys); err == nil {
		t.Error("subscription without auth key should be rejected")
	}
}
