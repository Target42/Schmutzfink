package httpapi

import (
	"net"
	"sync"
	"time"

	"schmutzfink/internal/auth"
)

const (
	loginMaxFails = 5
	loginLockFor  = 5 * time.Minute
)

type loginBucket struct {
	fails  int
	locked time.Time
}

type loginLimiter struct {
	mu     sync.Mutex
	byIP   map[string]*loginBucket
	byUser map[string]*loginBucket
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		byIP:   map[string]*loginBucket{},
		byUser: map[string]*loginBucket{},
	}
}

func (l *loginLimiter) allow(ip, username string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked()
	return bucketFree(l.byIP, ip) && bucketFree(l.byUser, normUser(username))
}

func (l *loginLimiter) fail(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	bumpBucket(l.byIP, ip)
	bumpBucket(l.byUser, normUser(username))
}

func (l *loginLimiter) ok(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ip != "" {
		delete(l.byIP, ip)
	}
	if user := normUser(username); user != "" {
		delete(l.byUser, user)
	}
}

func bucketFree(m map[string]*loginBucket, key string) bool {
	if key == "" {
		return true
	}
	b := m[key]
	if b == nil {
		return true
	}
	if !b.locked.IsZero() && time.Now().Before(b.locked) {
		return false
	}
	if !b.locked.IsZero() {
		delete(m, key)
	}
	return true
}

func bumpBucket(m map[string]*loginBucket, key string) {
	if key == "" {
		return
	}
	b := m[key]
	if b == nil {
		b = &loginBucket{}
		m[key] = b
	}
	b.fails++
	if b.fails >= loginMaxFails {
		b.locked = time.Now().Add(loginLockFor)
	}
}

func (l *loginLimiter) gcLocked() {
	now := time.Now()
	gcBuckets(l.byIP, now)
	gcBuckets(l.byUser, now)
}

func gcBuckets(m map[string]*loginBucket, now time.Time) {
	if len(m) < 2000 {
		return
	}
	for k, b := range m {
		if !b.locked.IsZero() && now.After(b.locked) {
			delete(m, k)
		}
	}
	if len(m) > 5000 {
		clear(m)
	}
}

func normUser(username string) string {
	return auth.NormalizeUsername(username)
}

func clientIP(rAddr string) string {
	host, _, err := net.SplitHostPort(rAddr)
	if err != nil {
		return rAddr
	}
	return host
}

func passwordChangeBlocked(path string, mustChange bool) bool {
	if !mustChange {
		return false
	}
	switch path {
	case "/api/auth/me", "/api/auth/logout", "/api/auth/password":
		return false
	default:
		return true
	}
}
