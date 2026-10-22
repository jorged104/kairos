package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestNotFoundUsesCommonErrorFormat(t *testing.T) {
	r := NewRouter(discardLogger())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Error.Code != "not_found" || body.Error.Message == "" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

func TestMethodNotAllowedUsesCommonErrorFormat(t *testing.T) {
	r := NewRouter(discardLogger())
	r.Get("/thing", func(w http.ResponseWriter, _ *http.Request) {})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/thing", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if body := decodeError(t, rec); body.Error.Code != "method_not_allowed" {
		t.Fatalf("code = %q, want method_not_allowed", body.Error.Code)
	}
}

func TestPanicUsesCommonErrorFormat(t *testing.T) {
	r := NewRouter(discardLogger())
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := decodeError(t, rec); body.Error.Code != "internal_error" {
		t.Fatalf("code = %q, want internal_error", body.Error.Code)
	}
}

func TestRequestIsLoggedWithMethodPathStatusAndDuration(t *testing.T) {
	var buf bytes.Buffer
	r := NewRouter(slog.New(slog.NewJSONHandler(&buf, nil)))
	r.Get("/teapot", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/teapot", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not JSON: %v (%s)", err, buf.String())
	}
	if entry["method"] != "GET" || entry["path"] != "/teapot" || entry["status"] != float64(http.StatusTeapot) {
		t.Fatalf("unexpected log entry: %v", entry)
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatalf("log entry without duration_ms: %v", entry)
	}
}
