package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// maxJSONBody bounds request bodies for JSON endpoints.
const maxJSONBody = 64 << 10

// apiError is the JSON error envelope: {"error":{"code":"…","message":"…"}}.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("write json", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message}})
}

// readJSON decodes a size-limited JSON body into v, rejecting unknown fields
// and trailing data. On failure it writes the response and returns false.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body is too large.")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.")
		}
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body has trailing data.")
		return false
	}
	return true
}
