// Package httpx contains JSON response helpers shared by every HTTP handler.
package httpx

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the project's common error format:
// {"error": {"code": "...", "message": "..."}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// WriteErrorDetails is WriteError with the optional "details" object, used to
// point at the invalid field or attribute.
func WriteErrorDetails(w http.ResponseWriter, status int, code, message string, details map[string]string) {
	WriteJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message, Details: details}})
}
