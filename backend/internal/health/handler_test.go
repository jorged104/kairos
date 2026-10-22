package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type fakePinger struct {
	calls int
	err   error
	delay time.Duration
}

func (f *fakePinger) Ping(ctx context.Context) error {
	f.calls++
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

func serve(t *testing.T, p Pinger, timeout time.Duration, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	NewHandler(NewService(p, timeout), slog.New(slog.NewJSONHandler(io.Discard, nil))).Routes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestLiveRespondsOKWithoutTouchingDatabase(t *testing.T) {
	p := &fakePinger{err: errors.New("down")}
	rec := serve(t, p, DatabaseTimeout, "/health/live")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := decode(t, rec); body["status"] != "ok" {
		t.Fatalf("unexpected body: %v", body)
	}
	if p.calls != 0 {
		t.Fatalf("live pinged the database %d times, want 0", p.calls)
	}
}

func TestReadyReportsDatabaseUp(t *testing.T) {
	rec := serve(t, &fakePinger{}, DatabaseTimeout, "/health/ready")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := decode(t, rec); body["status"] != "ok" || body["database"] != "up" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestReadyReportsDatabaseUnavailableOnError(t *testing.T) {
	rec := serve(t, &fakePinger{err: errors.New("connection refused")}, DatabaseTimeout, "/health/ready")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if code := errorCode(decode(t, rec)); code != "database_unavailable" {
		t.Fatalf("code = %q, want database_unavailable", code)
	}
}

func TestReadyReportsDatabaseUnavailableOnTimeout(t *testing.T) {
	// Scaled down: a 20ms timeout against a 1s ping stands in for 2s vs slower.
	start := time.Now()
	rec := serve(t, &fakePinger{delay: time.Second}, 20*time.Millisecond, "/health/ready")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if code := errorCode(decode(t, rec)); code != "database_unavailable" {
		t.Fatalf("code = %q, want database_unavailable", code)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("ready took %v, want it to give up at the timeout", elapsed)
	}
}

func TestDatabaseTimeoutIsTwoSeconds(t *testing.T) {
	if DatabaseTimeout != 2*time.Second {
		t.Fatalf("DatabaseTimeout = %v, want 2s (RF-15)", DatabaseTimeout)
	}
}
