package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MaxLoginFailures = 5
	LoginWindow      = 15 * time.Minute
	LoginBlock       = 15 * time.Minute
)

// RateLimiter blocks a key (an email or an IP) for LoginBlock once it has
// MaxLoginFailures failures inside LoginWindow (RF-9). It lives in memory:
// enough for a single API instance.
type RateLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	failures map[string][]time.Time
	blocked  map[string]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{now: time.Now, failures: map[string][]time.Time{}, blocked: map[string]time.Time{}}
}

// Allowed reports whether none of the keys is blocked.
func (l *RateLimiter) Allowed(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range keys {
		if until, ok := l.blocked[k]; ok {
			if now.Before(until) {
				return false
			}
			delete(l.blocked, k)
		}
	}
	return true
}

// Fail records a failed attempt for every key.
func (l *RateLimiter) Fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range keys {
		recent := l.failures[k][:0]
		for _, t := range l.failures[k] {
			if now.Sub(t) < LoginWindow {
				recent = append(recent, t)
			}
		}
		recent = append(recent, now)
		if len(recent) >= MaxLoginFailures {
			l.blocked[k] = now.Add(LoginBlock)
			delete(l.failures, k)
			continue
		}
		l.failures[k] = recent
	}
}

// Reset forgets the failures of a key after a successful login.
func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

func emailKey(email string) string { return "email:" + email }
func ipKey(ip string) string       { return "ip:" + ip }

// clientIP takes X-Real-IP (set by nginx) only when the proxy is trusted.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
