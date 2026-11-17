package handlers

import (
	"testing"

	"github.com/bigbrother/api/internal/config"
)

func TestMediaMTXAuthorize(t *testing.T) {
	h := NewMediaMTXAuthHandler(&config.Config{
		InternalToken:     "internal-0123456789-0123456789",
		MediaPublishToken: "publish-0123456789-0123456789",
		MediaReadToken:    "read-0123456789-0123456789abcd",
	})

	cases := []struct {
		name string
		req  MediaMTXAuthRequest
		want bool
	}{
		{"publish without credentials", MediaMTXAuthRequest{Action: "publish", Path: "salon"}, false},
		{"publish with publish token (rtsp user:pass)", MediaMTXAuthRequest{Action: "publish", Path: "salon", User: "publisher", Password: "publish-0123456789-0123456789"}, true},
		{"publish with publish token (?token=)", MediaMTXAuthRequest{Action: "publish", Path: "salon", Token: "publish-0123456789-0123456789"}, true},
		{"publish with bearer user:pass", MediaMTXAuthRequest{Action: "publish", Path: "salon", Token: "publisher:publish-0123456789-0123456789"}, true},
		{"publish with internal token", MediaMTXAuthRequest{Action: "publish", Path: "salon", Password: "internal-0123456789-0123456789"}, true},
		{"publish with read token", MediaMTXAuthRequest{Action: "publish", Path: "salon", Password: "read-0123456789-0123456789abcd"}, false},
		{"publish to invalid path", MediaMTXAuthRequest{Action: "publish", Path: "a/../b", Password: "publish-0123456789-0123456789"}, false},
		{"read without credentials", MediaMTXAuthRequest{Action: "read", Path: "salon"}, false},
		{"read with read token", MediaMTXAuthRequest{Action: "read", Path: "salon", User: "reader", Password: "read-0123456789-0123456789abcd"}, true},
		{"read with publish token", MediaMTXAuthRequest{Action: "read", Path: "salon", Password: "publish-0123456789-0123456789"}, false},
		{"playback with internal token", MediaMTXAuthRequest{Action: "playback", Path: "salon", Password: "internal-0123456789-0123456789"}, true},
		{"api with read token", MediaMTXAuthRequest{Action: "api", Password: "read-0123456789-0123456789abcd"}, false},
		{"api with internal token", MediaMTXAuthRequest{Action: "api", User: "api", Password: "internal-0123456789-0123456789"}, true},
		{"unknown action", MediaMTXAuthRequest{Action: "admin", Password: "internal-0123456789-0123456789"}, false},
	}
	for _, tc := range cases {
		if got := h.Authorize(&tc.req); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMediaMTXAuthorizeUnsetTokensNeverMatchEmpty(t *testing.T) {
	h := NewMediaMTXAuthHandler(&config.Config{InternalToken: "internal-0123456789-0123456789"})
	if h.Authorize(&MediaMTXAuthRequest{Action: "publish", Path: "salon"}) {
		t.Fatal("empty credential must not match an unset publish token")
	}
	if h.Authorize(&MediaMTXAuthRequest{Action: "read", Path: "salon", Password: ""}) {
		t.Fatal("empty credential must not match an unset read token")
	}
}
