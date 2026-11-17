package httpapi

import (
	"net/http"
	"testing"
)

func TestTrustProxy(t *testing.T) {
	cases := []struct {
		mode, addr string
		want       bool
	}{
		{mode: "1", addr: "203.0.113.9:1", want: true},
		{mode: "0", addr: "127.0.0.1:1", want: false},
		{mode: "auto", addr: "127.0.0.1:9", want: true},
		{mode: "auto", addr: "[::1]:9", want: true},
		{mode: "auto", addr: "203.0.113.9:9", want: false},
		{mode: "", addr: "10.0.0.2:9", want: false},
	}
	for _, tc := range cases {
		if got := trustProxy(tc.mode, tc.addr); got != tc.want {
			t.Errorf("trustProxy(%q, %q) = %v, want %v", tc.mode, tc.addr, got, tc.want)
		}
	}
}

func TestForwardedClientIP(t *testing.T) {
	h := make(http.Header)
	h.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.2")
	if got := forwardedClientIP(h); got != "203.0.113.10" {
		t.Fatalf("got %q", got)
	}
	h.Set("X-Real-IP", "198.51.100.4")
	if got := forwardedClientIP(h); got != "198.51.100.4" {
		t.Fatalf("real-ip got %q", got)
	}
	h.Set("X-Real-IP", "not-an-ip")
	h.Del("X-Forwarded-For")
	if got := forwardedClientIP(h); got != "" {
		t.Fatalf("invalid ip got %q", got)
	}
}
