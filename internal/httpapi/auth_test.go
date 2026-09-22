package httpapi

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"schmutzfink/internal/auth"
)

func TestCookieSecure(t *testing.T) {
	httpsReq := withProxyTrust(&http.Request{Header: make(http.Header)}, true)
	httpsReq.Header.Set("X-Forwarded-Proto", "https")

	untrusted := withProxyTrust(&http.Request{Header: make(http.Header)}, false)
	untrusted.Header.Set("X-Forwarded-Proto", "https")

	chainReq := withProxyTrust(&http.Request{Header: make(http.Header)}, true)
	chainReq.Header.Set("X-Forwarded-Proto", "https, http")

	httpReq := withProxyTrust(&http.Request{Header: make(http.Header)}, true)
	httpReq.Header.Set("X-Forwarded-Proto", "http")

	plain := &http.Request{Header: make(http.Header)}
	tlsReq := &http.Request{TLS: &tls.ConnectionState{}}

	cases := []struct {
		mode string
		r    *http.Request
		want bool
	}{
		{mode: "auto", r: httpsReq, want: true},
		{mode: "auto", r: untrusted, want: false},
		{mode: "", r: chainReq, want: true},
		{mode: "auto", r: httpReq, want: false},
		{mode: "auto", r: plain, want: false},
		{mode: "auto", r: tlsReq, want: true},
		{mode: "1", r: plain, want: true},
		{mode: "0", r: httpsReq, want: false},
	}
	for _, tc := range cases {
		if got := cookieSecure(tc.mode, tc.r); got != tc.want {
			t.Errorf("cookieSecure(%q) = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func withProxyTrust(r *http.Request, trusted bool) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), trustedProxyKey, trusted))
}

func TestAnonymousAPIIsUnauthorized(t *testing.T) {
	h := (&Server{}).Router()
	paths := []string{
		"/api/auth/me",
		"/api/records",
		"/api/records/map",
		"/api/records/export",
		"/api/records/summary",
		"/api/tokens",
		"/api/users",
		"/api/audit",
		"/api/deletion-requests",
		"/api/cases",
		"/api/embeddings",
		"/api/records/00000000-0000-4000-8000-000000000001/thumb",
	}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", path, rec.Code)
		}
	}
}

func TestAnonymousUserWritesAreUnauthorized(t *testing.T) {
	h := (&Server{}).Router()
	reqs := []*http.Request{
		httptest.NewRequest(http.MethodPatch, "/api/users/00000000-0000-4000-8000-000000000001", strings.NewReader(`{"disabled":true}`)),
		httptest.NewRequest(http.MethodPost, "/api/users/00000000-0000-4000-8000-000000000001/password", strings.NewReader(`{"password":"ein-gutes-kennwort"}`)),
		httptest.NewRequest(http.MethodDelete, "/api/records/00000000-0000-4000-8000-000000000001", nil),
		httptest.NewRequest(http.MethodPost, "/api/records/00000000-0000-4000-8000-000000000001/deletion-request", strings.NewReader(`{"reason":"test"}`)),
		httptest.NewRequest(http.MethodPost, "/api/search/image", nil),
	}
	for _, req := range reqs {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestPresentUserIncludesDisabled(t *testing.T) {
	got := presentUser(auth.Principal{
		UserID:    "1",
		Username:  "anna",
		Role:      auth.RoleUser,
		CanDelete: true,
		Disabled:  true,
	})
	if got["disabled"] != true {
		t.Fatalf("disabled: %v", got["disabled"])
	}
	if got["can_delete"] != true {
		t.Fatalf("can_delete: %v", got["can_delete"])
	}
}

func TestPresentUserHidesDeleteFlagForSearcher(t *testing.T) {
	got := presentUser(auth.Principal{
		Role:      auth.RoleSearcher,
		CanDelete: true,
	})
	if got["can_delete"] != false {
		t.Fatalf("can_delete: %v", got["can_delete"])
	}
}

func TestRequireWriterRejectsSearcher(t *testing.T) {
	s := &Server{}
	h := s.requireWriter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/records", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey, auth.Principal{Role: auth.RoleSearcher}))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", rec.Code)
	}
}

func TestRequireWriterAllowsClerk(t *testing.T) {
	s := &Server{}
	h := s.requireWriter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/records", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey, auth.Principal{Role: auth.RoleUser}))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
}

func TestRequireDeletionOfficer(t *testing.T) {
	s := &Server{}
	h := s.requireDeletionOfficer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/deletion-requests", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey, auth.Principal{Role: auth.RoleAdmin}))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin without flag: got %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/deletion-requests", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey, auth.Principal{Role: auth.RoleUser, CanDelete: true}))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("officer: got %d, want 204", rec.Code)
	}
}

func TestHealthIsPublic(t *testing.T) {
	h := (&Server{}).Router()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health: got %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing frame deny")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS must not be set on plain HTTP")
	}
}

func TestHSTSOnHTTPS(t *testing.T) {
	h := (&Server{}).Router()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.TLS = &tls.ConnectionState{}
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS missing on TLS")
	}
}
