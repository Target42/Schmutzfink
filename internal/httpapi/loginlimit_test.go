package httpapi

import (
	"testing"
	"time"
)

func TestPasswordChangeBlocked(t *testing.T) {
	if passwordChangeBlocked("/api/records", false) {
		t.Fatal("without flag nothing is blocked")
	}
	if passwordChangeBlocked("/api/auth/me", true) {
		t.Fatal("me must stay allowed")
	}
	if passwordChangeBlocked("/api/auth/password", true) {
		t.Fatal("password change must stay allowed")
	}
	if passwordChangeBlocked("/api/auth/logout", true) {
		t.Fatal("logout must stay allowed")
	}
	if !passwordChangeBlocked("/api/records", true) {
		t.Fatal("records must be blocked until password change")
	}
	if !passwordChangeBlocked("/api/records/x/thumb", true) {
		t.Fatal("thumbs must be blocked until password change")
	}
}

func TestLoginLimiter(t *testing.T) {
	l := newLoginLimiter()
	if !l.allow("1.2.3.4", "anna") {
		t.Fatal("fresh IP should be allowed")
	}
	for i := 0; i < loginMaxFails; i++ {
		l.fail("1.2.3.4", "anna")
	}
	if l.allow("1.2.3.4", "berta") {
		t.Fatal("IP should be locked after too many failures")
	}
	if !l.allow("8.8.8.8", "berta") {
		t.Fatal("other IP should still be allowed")
	}
	l.byIP["1.2.3.4"].locked = time.Now().Add(-time.Second)
	if !l.allow("1.2.3.4", "berta") {
		t.Fatal("lock should expire")
	}
	l.ok("8.8.8.8", "berta")
	l.fail("9.9.9.9", "clara")
	l.ok("9.9.9.9", "clara")
	if !l.allow("9.9.9.9", "clara") {
		t.Fatal("successful login should clear failures")
	}
}

func TestLoginLimiterLocksUsername(t *testing.T) {
	l := newLoginLimiter()
	for i := 0; i < loginMaxFails; i++ {
		l.fail("1.2.3.4", "Admin")
	}
	if l.allow("8.8.8.8", "admin") {
		t.Fatal("username should be locked across IPs")
	}
	if !l.allow("8.8.8.8", "other") {
		t.Fatal("other username should still be allowed")
	}
}

func TestLockedIPsAndUnlock(t *testing.T) {
	l := newLoginLimiter()
	for i := 0; i < loginMaxFails; i++ {
		l.fail("1.2.3.4", "anna")
		l.fail("2001:db8::1", "eva")
	}
	l.fail("8.8.8.8", "berta")
	l.byIP["5.5.5.5"] = &loginBucket{fails: loginMaxFails, locked: time.Now().Add(-time.Second)}

	now := time.Now()
	got := l.lockedIPs(now)
	if len(got) != 2 {
		t.Fatalf("locked IPs: %+v", got)
	}
	if got[0].IP != "1.2.3.4" || got[1].IP != "2001:db8::1" {
		t.Fatalf("order: %+v", got)
	}
	if got[0].Fails != loginMaxFails || !got[0].LockedUntil.After(now) {
		t.Fatalf("lock: %+v", got[0])
	}
	if _, ok := l.byIP["5.5.5.5"]; ok {
		t.Fatal("expired lock should be dropped")
	}
	if !l.unlockIP("1.2.3.4") {
		t.Fatal("unlock should succeed")
	}
	if l.unlockIP("1.2.3.4") || l.unlockIP("8.8.8.8") || l.unlockIP("  ") {
		t.Fatal("unlock should fail when the IP is not locked")
	}
	if !l.allow("1.2.3.4", "other") {
		t.Fatal("unlocked IP should accept another username")
	}
	if l.allow("9.9.9.9", "anna") {
		t.Fatal("username lock should remain")
	}
}

func TestClientIP(t *testing.T) {
	if got := clientIP("192.0.2.1:54321"); got != "192.0.2.1" {
		t.Errorf("got %q", got)
	}
	if got := clientIP("2001:db8::1"); got != "2001:db8::1" {
		t.Errorf("got %q", got)
	}
}
