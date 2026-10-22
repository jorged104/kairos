package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestRunFailsWhenDatabaseIsUnreachable(t *testing.T) {
	// Port 1 on localhost has nothing listening, so the startup ping fails.
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")

	err := run(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("run returned nil, want an error when the database is unreachable")
	}
}

func TestRunFailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if err := run(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil))); err == nil {
		t.Fatal("run returned nil, want an error when DATABASE_URL is missing")
	}
}
