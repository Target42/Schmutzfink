package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type proxyCtxKey int

const trustedProxyKey proxyCtxKey = 1

func (s *Server) withClientAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trusted := trustProxy(s.TrustProxy, r.RemoteAddr)
		if trusted {
			if ip := forwardedClientIP(r.Header); ip != "" {
				_, port, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					port = "0"
				}
				r.RemoteAddr = net.JoinHostPort(ip, port)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), trustedProxyKey, trusted)))
	})
}

func trustProxy(mode, remoteAddr string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "1", "true", "yes", "always":
		return true
	case "0", "false", "no", "never":
		return false
	}
	return isLoopbackAddr(remoteAddr)
}

func isLoopbackAddr(rAddr string) bool {
	host := clientIP(rAddr)
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func forwardedClientIP(h http.Header) string {
	if ip := strings.TrimSpace(h.Get("X-Real-IP")); ip != "" {
		return takeFirstForwarded(ip)
	}
	if ip := strings.TrimSpace(h.Get("X-Forwarded-For")); ip != "" {
		return takeFirstForwarded(ip)
	}
	return ""
}

func takeFirstForwarded(raw string) string {
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	ip := strings.TrimSpace(raw)
	if net.ParseIP(strings.Trim(ip, "[]")) == nil {
		return ""
	}
	return ip
}

func proxyTrusted(r *http.Request) bool {
	if r == nil {
		return false
	}
	v, _ := r.Context().Value(trustedProxyKey).(bool)
	return v
}
