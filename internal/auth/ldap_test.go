package auth

import (
	"context"
	"errors"
	"testing"
)

func TestDomainTemplate(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "", want: "{username}"},
		{in: " stadt.local ", want: "{username}@stadt.local"},
		{in: "STADT", want: `STADT\{username}`},
	}
	for _, tc := range cases {
		if got := DomainTemplate(tc.in); got != tc.want {
			t.Errorf("DomainTemplate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBindIdentity(t *testing.T) {
	cases := []struct {
		template, user, want string
	}{
		{template: "{username}@stadt.local", user: "j.mueller", want: "j.mueller@stadt.local"},
		{template: `STADT\{username}`, user: "j.mueller", want: `STADT\j.mueller`},
		{template: "{username}@stadt.local", user: "j.mueller@stadt.local", want: "j.mueller@stadt.local"},
		{template: "{username}@stadt.local", user: `STADT\j.mueller`, want: `STADT\j.mueller`},
		{template: "", user: "j.mueller", want: "j.mueller"},
		{template: "{username}@stadt.local", user: "  ", want: ""},
	}
	for _, tc := range cases {
		if got := BindIdentity(tc.template, tc.user); got != tc.want {
			t.Errorf("BindIdentity(%q, %q) = %q, want %q", tc.template, tc.user, got, tc.want)
		}
	}
}

type fakeDirectory struct {
	user string
	pass string
	err  error
}

func (f fakeDirectory) Authenticate(_ context.Context, username, password string) error {
	if f.err != nil {
		return f.err
	}
	if username == f.user && password == f.pass {
		return nil
	}
	return ErrInvalidCredentials
}

func TestCheckPassword(t *testing.T) {
	ctx := context.Background()
	hash := mustHash("ein-gutes-kennwort")
	if err := checkPassword(ctx, nil, ProviderLocal, "anna", hash, "ein-gutes-kennwort"); err != nil {
		t.Fatalf("local ok: %v", err)
	}
	if err := checkPassword(ctx, nil, ProviderLocal, "anna", hash, "falsch"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("local bad: %v", err)
	}

	dir := fakeDirectory{user: "anna", pass: "AdKennwort1"}
	if err := checkPassword(ctx, dir, ProviderLDAP, "anna", "", "AdKennwort1"); err != nil {
		t.Fatalf("ldap ok: %v", err)
	}
	if err := checkPassword(ctx, dir, ProviderLDAP, "anna", "", "falsch"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ldap bad: %v", err)
	}
	if err := checkPassword(ctx, nil, ProviderLDAP, "anna", "", "AdKennwort1"); !errors.Is(err, ErrDirectoryUnavailable) {
		t.Fatalf("ldap down: %v", err)
	}
}

func TestNormalizeProvider(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{in: "", want: ProviderLocal, ok: true},
		{in: "local", want: ProviderLocal, ok: true},
		{in: "LDAP", want: ProviderLDAP, ok: true},
		{in: "ad", want: ProviderLDAP, ok: true},
		{in: "oidc", ok: false},
	}
	for _, tc := range cases {
		got, ok := NormalizeProvider(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("NormalizeProvider(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidUsername(t *testing.T) {
	if _, err := ValidUsername("j.mueller"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidUsername("  "); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("blank: %v", err)
	}
	if _, err := ValidUsername("a/b"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("slash: %v", err)
	}
}
