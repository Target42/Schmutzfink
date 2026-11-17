package auth

import "testing"

func TestNormalizeTokenName(t *testing.T) {
	if _, err := NormalizeTokenName("  "); err != ErrTokenName {
		t.Fatal("empty")
	}
	if _, err := NormalizeTokenName("Skript"); err != nil {
		t.Fatal(err)
	}
	long := make([]rune, APITokenNameMax+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := NormalizeTokenName(string(long)); err != ErrTokenName {
		t.Fatal("too long")
	}
	if _, err := NormalizeTokenName("bad\nname"); err != ErrTokenName {
		t.Fatal("control")
	}
}

func TestNormalizeTokenExpiryDays(t *testing.T) {
	if _, err := NormalizeTokenExpiryDays(7); err != ErrTokenExpiry {
		t.Fatal("7 days")
	}
	got, err := NormalizeTokenExpiryDays(90)
	if err != nil || got != 90 {
		t.Fatalf("%d %v", got, err)
	}
}

func TestTokenPrefix(t *testing.T) {
	got := tokenPrefix("sft_0123456789abcdef")
	if got != "sft_01234567" {
		t.Fatalf("%q", got)
	}
}
