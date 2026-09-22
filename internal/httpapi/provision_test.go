package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"schmutzfink/internal/auth"
)

func TestProvisionDisabledIsNotFound(t *testing.T) {
	h := (&Server{}).Router()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/provision/users", strings.NewReader(`{"username":"anna"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestProvisionShortTokenIsNotFound(t *testing.T) {
	h := (&Server{ProvisionToken: "kurz"}).Router()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/provision/users", strings.NewReader(`{"username":"anna"}`))
	req.Header.Set("Authorization", "Bearer kurz")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestProvisionUnauthorized(t *testing.T) {
	h := (&Server{ProvisionToken: "ein-langes-provision-token"}).Router()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/provision/users", strings.NewReader(`{"username":"anna"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: got %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/provision/users", strings.NewReader(`{"username":"anna"}`))
	req.Header.Set("Authorization", "Bearer anderes-langes-token!")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: got %d, want 401", rec.Code)
	}
}

func TestProvisionBadUsername(t *testing.T) {
	h := (&Server{ProvisionToken: "ein-langes-provision-token", Auth: &auth.Service{}}).Router()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/provision/users", strings.NewReader(`{"username":""}`))
	req.Header.Set("Authorization", "Bearer ein-langes-provision-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestTokenEqual(t *testing.T) {
	if !tokenEqual("abc", "abc") || tokenEqual("abc", "abd") || tokenEqual("ab", "abc") || tokenEqual("", "") {
		t.Fatal("tokenEqual")
	}
}

func TestPresentUserIncludesProvider(t *testing.T) {
	got := presentUser(auth.Principal{Username: "anna", AuthProvider: auth.ProviderLDAP})
	if got["auth_provider"] != auth.ProviderLDAP {
		t.Fatalf("provider: %v", got["auth_provider"])
	}
	got = presentUser(auth.Principal{Username: "admin"})
	if got["auth_provider"] != auth.ProviderLocal {
		t.Fatalf("default provider: %v", got["auth_provider"])
	}
}
