package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func setAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_MODE", "local")
	t.Setenv("SESSION_SECRET", "test-session-secret-0123456789abcdef")
	t.Setenv("PUBLIC_URL", "http://localhost:5173")
}

func TestRunFailsWhenDatabaseIsUnreachable(t *testing.T) {
	setAuthEnv(t)
	// Port 1 on localhost has nothing listening, so the startup ping fails.
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")

	err := run(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("run returned nil, want an error when the database is unreachable")
	}
}

func TestRunFailsWithoutDatabaseURL(t *testing.T) {
	setAuthEnv(t)
	t.Setenv("DATABASE_URL", "")

	if err := run(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil))); err == nil {
		t.Fatal("run returned nil, want an error when DATABASE_URL is missing")
	}
}

func TestRunFailsWithInvalidAuthMode(t *testing.T) {
	setAuthEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	t.Setenv("AUTH_MODE", "ldap")

	err := run(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "AUTH_MODE") {
		t.Fatalf("err = %v, want an AUTH_MODE error before touching the database", err)
	}
}
