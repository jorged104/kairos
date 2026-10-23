package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBlocksAfterFiveFailuresForFifteenMinutes(t *testing.T) {
	now := time.Now()
	l := NewRateLimiter()
	l.now = func() time.Time { return now }

	for range MaxLoginFailures - 1 {
		l.Fail("k")
	}
	if !l.Allowed("k") {
		t.Fatal("blocked after 4 failures")
	}
	l.Fail("k")
	if l.Allowed("k") || l.Allowed("other", "k") {
		t.Fatal("not blocked after 5 failures")
	}
	now = now.Add(LoginBlock - time.Second)
	if l.Allowed("k") {
		t.Fatal("unblocked before 15 minutes")
	}
	now = now.Add(2 * time.Second)
	if !l.Allowed("k") {
		t.Fatal("still blocked after 15 minutes")
	}
}

func TestRateLimiterForgetsOldFailures(t *testing.T) {
	now := time.Now()
	l := NewRateLimiter()
	l.now = func() time.Time { return now }
	for range MaxLoginFailures - 1 {
		l.Fail("k")
	}
	now = now.Add(LoginWindow + time.Second)
	l.Fail("k")
	if !l.Allowed("k") {
		t.Fatal("failures outside the 15-minute window counted")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Real-IP", "203.0.113.9")
	if got := clientIP(r, false); got != "10.0.0.1" {
		t.Errorf("untrusted proxy: %q", got)
	}
	if got := clientIP(r, true); got != "203.0.113.9" {
		t.Errorf("trusted proxy: %q", got)
	}
}
