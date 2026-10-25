package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"schmutzfink/internal/auth"
)

func TestBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/records", nil)
	if _, ok := bearerToken(req); ok {
		t.Fatal("missing header")
	}
	req.Header.Set("Authorization", "Bearer sft_abc")
	got, ok := bearerToken(req)
	if !ok || got != "sft_abc" {
		t.Fatalf("got %q %v", got, ok)
	}
	req.Header.Set("Authorization", "bearer sft_abc")
	got, ok = bearerToken(req)
	if !ok || got != "sft_abc" {
		t.Fatalf("case: %q %v", got, ok)
	}
	req.Header.Set("Authorization", "Basic abc")
	if tok, ok := bearerToken(req); !ok || tok != "" {
		t.Fatalf("non-bearer: %q %v", tok, ok)
	}
	req.Header.Set("Authorization", "Bearer ")
	if tok, ok := bearerToken(req); !ok || tok != "" {
		t.Fatalf("empty bearer: %q %v", tok, ok)
	}
}

func TestTokenPathAllowed(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	allowed := [][2]string{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/records"},
		{http.MethodGet, "/api/records/map"},
		{http.MethodGet, "/api/records/export"},
		{http.MethodGet, "/api/records/summary"},
		{http.MethodPost, "/api/records/summary"},
		{http.MethodGet, "/api/records/" + id},
		{http.MethodGet, "/api/motifs"},
		{http.MethodGet, "/api/motifs/" + id},
		{http.MethodGet, "/api/motifs/" + id + "/suggestions"},
		{http.MethodGet, "/api/cases"},
		{http.MethodGet, "/api/cases/" + id},
		{http.MethodGet, "/api/fields"},
		{http.MethodGet, "/api/embeddings"},
		{http.MethodGet, "/api/geocode"},
		{http.MethodGet, "/api/geocode/reverse"},
		{http.MethodGet, "/api/sightings/" + id + "/similar"},
		{http.MethodPost, "/api/records/export"},
		{http.MethodPost, "/api/records/summary"},
		{http.MethodPost, "/api/search/image"},
	}
	for _, tc := range allowed {
		if !tokenPathAllowed(tc[0], tc[1]) {
			t.Errorf("should allow %s %s", tc[0], tc[1])
		}
	}
	denied := [][2]string{
		{http.MethodGet, "/api/tokens"},
		{http.MethodPost, "/api/tokens"},
		{http.MethodGet, "/api/users"},
		{http.MethodGet, "/api/audit"},
		{http.MethodGet, "/api/records/" + id + "/original"},
		{http.MethodGet, "/api/records/" + id + "/thumb"},
		{http.MethodGet, "/api/sightings/" + id + "/thumb"},
		{http.MethodPost, "/api/records"},
		{http.MethodPost, "/api/cases"},
		{http.MethodPost, "/api/cases/" + id + "/close"},
		{http.MethodPatch, "/api/records/" + id},
		{http.MethodDelete, "/api/records/" + id},
		{http.MethodPost, "/api/auth/password"},
		{http.MethodPost, "/api/provision/users"},
		{http.MethodGet, "/api/provision/users/anna"},
		{http.MethodPatch, "/api/provision/users/anna"},
	}
	for _, tc := range denied {
		if tokenPathAllowed(tc[0], tc[1]) {
			t.Errorf("should deny %s %s", tc[0], tc[1])
		}
	}
}

func TestAnonymousTokensAreUnauthorized(t *testing.T) {
	h := (&Server{}).Router()
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/tokens", nil),
		httptest.NewRequest(http.MethodPost, "/api/tokens", nil),
		httptest.NewRequest(http.MethodDelete, "/api/tokens/00000000-0000-4000-8000-000000000001", nil),
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestPresentAPITokenOmitsSecret(t *testing.T) {
	got := presentAPIToken(auth.APIToken{
		ID:        "t1",
		Name:      "Skript",
		Prefix:    "sft_abcd1234",
		ExpiresAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	})
	if _, ok := got["token"]; ok {
		t.Fatal("list presenter must not include token")
	}
	if got["prefix"] != "sft_abcd1234" || got["name"] != "Skript" {
		t.Fatalf("%v", got)
	}
}
