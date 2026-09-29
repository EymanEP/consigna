package server

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/EymanEP/consigna/internal/store"
)

// This file implements the parts of the tus 1.0 resumable upload protocol
// (https://tus.io/protocols/resumable-upload) the web UI uses: the core
// protocol plus the creation and termination extensions.

const (
	tusVersion       = "1.0.0"
	tusExtensions    = "creation,termination"
	tusContentType   = "application/offset+octet-stream"
	uploadsPathLabel = "/api/v1/uploads/"
)

func tusHeaders(w http.ResponseWriter) {
	w.Header().Set("Tus-Resumable", tusVersion)
	w.Header().Set("Cache-Control", "no-store")
}

// tusVersionOK rejects requests that do not speak tus 1.0.0.
func tusVersionOK(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Tus-Resumable") != tusVersion {
		w.Header().Set("Tus-Version", tusVersion)
		writeError(w, http.StatusPreconditionFailed, "tus_version", "Unsupported tus version.")
		return false
	}
	return true
}

func (s *Server) handleTusOptions(w http.ResponseWriter, _ *http.Request) {
	tusHeaders(w)
	w.Header().Set("Tus-Version", tusVersion)
	w.Header().Set("Tus-Extension", tusExtensions)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTusCreate(w http.ResponseWriter, r *http.Request) {
	tusHeaders(w)
	if !tusVersionOK(w, r) {
		return
	}
	if r.Header.Get("Upload-Defer-Length") != "" {
		writeError(w, http.StatusBadRequest, "defer_length", "Upload length must be known in advance.")
		return
	}
	length, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil || length < 0 {
		writeError(w, http.StatusBadRequest, "invalid_length", "Upload-Length is missing or invalid.")
		return
	}
	meta := parseMetadata(r.Header.Get("Upload-Metadata"))
	name := meta["filename"]
	if name == "" {
		name = meta["name"]
	}
	d := deviceFrom(r)
	u, file, err := s.store.CreateUpload(store.Owner{ID: d.ID, Name: d.Name}, name, length)
	switch {
	case errors.Is(err, store.ErrQuotaExceeded):
		writeError(w, http.StatusRequestEntityTooLarge, "tray_full", "There is not enough space left in the tray for this file.")
		return
	case err != nil:
		s.log.Error("create upload", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Could not start the upload.")
		return
	}
	if file != nil {
		s.log.Info("file added", "file", file.ID, "size", file.Size, "by", d.Name)
	}
	w.Header().Set("Location", uploadsPathLabel+u.ID)
	w.Header().Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleTusHead(w http.ResponseWriter, r *http.Request) {
	tusHeaders(w)
	if !tusVersionOK(w, r) {
		return
	}
	u, err := s.store.GetUpload(r.PathValue("id"), deviceFrom(r).ID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(u.Size, 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTusPatch(w http.ResponseWriter, r *http.Request) {
	tusHeaders(w)
	if !tusVersionOK(w, r) {
		return
	}
	if ct, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";"); strings.TrimSpace(ct) != tusContentType {
		writeError(w, http.StatusUnsupportedMediaType, "content_type", "Content-Type must be "+tusContentType+".")
		return
	}
	offset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid_offset", "Upload-Offset is missing or invalid.")
		return
	}

	d := deviceFrom(r)
	body := newUploadBody(r.Body, http.NewResponseController(w), idleTimeout)
	u, file, err := s.store.WriteChunk(r.Context(), r.PathValue("id"), d.ID, offset, body, body.Interrupt)
	interrupted := body.Finish()
	if file != nil {
		s.log.Info("file added", "file", file.ID, "size", file.Size, "by", d.Name)
	}
	switch {
	case err == nil:
		w.Header().Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "upload_not_found", "This upload no longer exists.")
	case errors.Is(err, store.ErrOffsetMismatch):
		w.Header().Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
		writeError(w, http.StatusConflict, "offset_mismatch", "Upload-Offset does not match the server.")
	case errors.Is(err, store.ErrLocked):
		writeError(w, http.StatusLocked, "upload_busy", "This upload is busy. Retry shortly.")
	case errors.Is(err, store.ErrTooLarge):
		writeError(w, http.StatusBadRequest, "too_much_data", "More data was sent than the declared length.")
	case errors.Is(err, store.ErrClosed):
		writeError(w, http.StatusServiceUnavailable, "shutting_down", "The host is shutting down.")
	case errors.Is(err, context.Canceled) || interrupted:
		// The client went away or reconnected; nobody reads this response.
		w.WriteHeader(http.StatusRequestTimeout)
	default:
		s.log.Debug("upload chunk ended early", "upload", r.PathValue("id"), "offset", u.Offset, "err", err)
		writeError(w, http.StatusInternalServerError, "upload_interrupted", "The upload was interrupted.")
	}
}

func (s *Server) handleTusDelete(w http.ResponseWriter, r *http.Request) {
	tusHeaders(w)
	if !tusVersionOK(w, r) {
		return
	}
	if err := s.store.TerminateUpload(r.PathValue("id"), deviceFrom(r).ID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseMetadata decodes the Upload-Metadata header: comma-separated pairs of
// a key and an optional base64 value. Malformed pairs are skipped.
func parseMetadata(header string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(header, ",") {
		key, val, _ := strings.Cut(strings.TrimSpace(pair), " ")
		if key == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(val))
		if err != nil {
			continue
		}
		out[key] = string(decoded)
	}
	return out
}
