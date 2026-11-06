// Package llm talks to an AI service compatible with OpenAI's chat API, only
// to check that it works (spec 020, RF-11 to RF-13). The reports themselves
// are written by the agent service (ADR 0004).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config is the AI service set in the system screen, with its key in clear.
type Config struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Model   string
}

// Ready reports whether reports can be generated with it (RF-16).
func (c Config) Ready() bool { return c.Enabled && c.BaseURL != "" && c.Model != "" }

// The steps a check can fail at (RF-13).
const (
	StepConnect = "connect" // the service did not answer
	StepAuth    = "auth"    // the key was rejected
	StepModel   = "model"   // the model does not exist
	StepTools   = "tools"   // the model does not call tools
)

// Error says where a check failed.
type Error struct {
	Step string
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("ia (%s): %v", e.Step, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// CheckTimeout bounds the whole check.
const CheckTimeout = 30 * time.Second

// Check asks the model for one call of a "ping" tool: it proves the address,
// the key, the model and that the model can make queries (RF-11).
func Check(ctx context.Context, client *http.Client, cfg Config) error {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"model":    cfg.Model,
		"messages": []map[string]string{{"role": "user", "content": "Llama a la herramienta ping."}},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name": "ping", "description": "Comprueba la conexión.",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
			},
		}},
		// No token cap: recent models reject max_tokens, and reasoning models
		// can spend a small cap before calling the tool.
		"tool_choice": "required",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint(cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return &Error{Step: StepConnect, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return &Error{Step: StepConnect, Err: err}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var out struct {
		Error   *apiError `json:"error"`
		Choices []struct {
			Message struct {
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	_ = json.Unmarshal(raw, &out)
	msg := ""
	if out.Error != nil {
		msg = strings.ToLower(out.Error.Message + " " + fmt.Sprint(out.Error.Code))
	}
	// The service's own message goes into the error for the log (not to the
	// browser): it is what tells apart a quota, a bad parameter or a bad URL.
	httpErr := fmt.Errorf("HTTP %d: %s", res.StatusCode, snippet(out.Error, raw))
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return &Error{Step: StepAuth, Err: httpErr}
	case res.StatusCode == http.StatusNotFound || strings.Contains(msg, "model_not_found") ||
		(strings.Contains(msg, "model") && (strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist"))):
		return &Error{Step: StepModel, Err: httpErr}
	case res.StatusCode >= 400 && (strings.Contains(msg, "tool") || strings.Contains(msg, "function")):
		return &Error{Step: StepTools, Err: httpErr}
	case res.StatusCode >= 300:
		return &Error{Step: StepConnect, Err: httpErr}
	case len(out.Choices) == 0:
		return &Error{Step: StepConnect, Err: errors.New("respuesta sin choices")}
	case len(out.Choices[0].Message.ToolCalls) == 0:
		return &Error{Step: StepTools, Err: errors.New("el modelo no llamó a la herramienta")}
	}
	return nil
}

// Base is the base address as the OpenAI clients expect it, also when the
// full chat completions endpoint was pasted.
func Base(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	return strings.TrimRight(strings.TrimSuffix(base, "/chat/completions"), "/")
}

// Endpoint is the chat completions URL of a base address.
func Endpoint(base string) string { return Base(base) + "/chat/completions" }

// apiError is the error object of an OpenAI-compatible answer.
type apiError struct {
	Message string `json:"message"`
	Code    any    `json:"code"`
}

// snippet is the service's error message, or the start of its body.
func snippet(e *apiError, raw []byte) string {
	s := string(raw)
	if e != nil && e.Message != "" {
		s = e.Message
	}
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
