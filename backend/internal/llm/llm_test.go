package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBase(t *testing.T) {
	for in, want := range map[string]string{
		"https://api.example.com/v1":                   "https://api.example.com/v1",
		" https://api.example.com/v1/ ":                "https://api.example.com/v1",
		"https://api.example.com/v1/chat/completions":  "https://api.example.com/v1",
		"https://api.example.com/v1/chat/completions/": "https://api.example.com/v1",
		"http://host.docker.internal:11434/v1":         "http://host.docker.internal:11434/v1",
	} {
		if got := Base(in); got != want {
			t.Errorf("Base(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheck(t *testing.T) {
	var body map[string]any
	answer := func(status int, out string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" {
				http.NotFound(w, r)
				return
			}
			body = nil
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(out))
		}))
	}
	cases := []struct {
		name, out, step string
		status          int
		path            string
	}{
		{"ok", `{"choices":[{"message":{"tool_calls":[{"id":"1"}]}}]}`, "", 200, "/v1"},
		{"full endpoint pasted", `{"choices":[{"message":{"tool_calls":[{"id":"1"}]}}]}`, "", 200, "/v1/chat/completions"},
		{"bad key", `{"error":{"message":"Incorrect API key"}}`, StepAuth, 401, "/v1"},
		{"no model", `{"error":{"message":"The model x does not exist","code":"model_not_found"}}`, StepModel, 404, "/v1"},
		{"no tools", `{"error":{"message":"tools are not supported"}}`, StepTools, 400, "/v1"},
		{"no tool call", `{"choices":[{"message":{"content":"pong"}}]}`, StepTools, 200, "/v1"},
		{"quota", `{"error":{"message":"You exceeded your current quota"}}`, StepConnect, 429, "/v1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := answer(c.status, c.out)
			defer srv.Close()
			err := Check(context.Background(), srv.Client(), Config{Enabled: true, BaseURL: srv.URL + c.path, Model: "m"})
			if c.step == "" {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				if _, ok := body["max_tokens"]; ok {
					t.Error("max_tokens sent: recent models reject it")
				}
				return
			}
			var le *Error
			if !errors.As(err, &le) || le.Step != c.step {
				t.Fatalf("err = %v, want step %s", err, c.step)
			}
			if c.status != 200 && !strings.Contains(le.Err.Error(), "HTTP ") || c.name == "quota" && !strings.Contains(le.Err.Error(), "quota") {
				t.Errorf("error without the service message: %v", le.Err)
			}
		})
	}
	var le *Error
	if err := Check(context.Background(), nil, Config{BaseURL: "http://127.0.0.1:1/v1", Model: "m"}); !errors.As(err, &le) || le.Step != StepConnect {
		t.Fatalf("unreachable: err = %v", err)
	}
}
