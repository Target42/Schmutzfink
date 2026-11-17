package auth

import "testing"

func TestIsAdmin(t *testing.T) {
	if !(Principal{Role: RoleAdmin}.IsAdmin()) {
		t.Fatal("admin role")
	}
	if (Principal{Role: RoleUser}.IsAdmin()) {
		t.Fatal("user role")
	}
	if (Principal{}.IsAdmin()) {
		t.Fatal("empty role")
	}
}

func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "admin", want: RoleAdmin, ok: true},
		{in: " USER ", want: RoleUser, ok: true},
		{in: "", want: RoleUser, ok: true},
		{in: "searcher", want: RoleSearcher, ok: true},
		{in: "gast", ok: false},
	}
	for _, tc := range cases {
		got, ok := NormalizeRole(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("NormalizeRole(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCapabilities(t *testing.T) {
	admin := Principal{Role: RoleAdmin, CanDelete: true}
	if !admin.CanWrite() || !admin.CanExecuteDeletion() {
		t.Fatal("admin with flag")
	}
	clerk := Principal{Role: RoleUser}
	if !clerk.CanWrite() || clerk.CanExecuteDeletion() {
		t.Fatal("clerk")
	}
	officer := Principal{Role: RoleUser, CanDelete: true}
	if !officer.CanExecuteDeletion() {
		t.Fatal("officer")
	}
	searcher := Principal{Role: RoleSearcher, CanDelete: true}
	if searcher.CanWrite() || searcher.CanExecuteDeletion() {
		t.Fatal("searcher must stay read-only")
	}
	token := Principal{Role: RoleAdmin, CanDelete: true, TokenID: "tok"}
	if token.IsAdmin() || token.CanWrite() || token.CanExecuteDeletion() || !token.ViaAPIToken() {
		t.Fatal("api token must stay read-only")
	}
	if !(Principal{}.Local()) || (Principal{AuthProvider: ProviderLDAP}.Local()) {
		t.Fatal("local vs ldap")
	}
}

func TestNormalizeUsername(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "Admin", want: "admin"},
		{in: " ADMIN ", want: "admin"},
		{in: "Stefan", want: "stefan"},
	}
	for _, tc := range cases {
		if got := NormalizeUsername(tc.in); got != tc.want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWeakPassword(t *testing.T) {
	cases := []struct {
		user, pass string
		weak       bool
	}{
		{user: "admin", pass: "admin", weak: true},
		{user: "admin", pass: "passwort", weak: true},
		{user: "stefan", pass: "stefan", weak: true},
		{user: "stefan", pass: "kurz", weak: true},
		{user: "admin", pass: "CHANGE_ME", weak: true},
		{user: "admin", pass: "ein-gutes-kennwort", weak: false},
	}
	for _, tc := range cases {
		if got := WeakPassword(tc.user, tc.pass); got != tc.weak {
			t.Errorf("WeakPassword(%q, %q) = %v, want %v", tc.user, tc.pass, got, tc.weak)
		}
	}
}
