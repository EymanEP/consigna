package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/EymanEP/consigna/internal/qr"
	"github.com/EymanEP/consigna/internal/session"
	"github.com/EymanEP/consigna/internal/settings"
)

// handleHostQR renders the join link for one of this machine's addresses.
// Only addresses the server itself reports are accepted, so the endpoint
// cannot be used to make arbitrary QR codes.
func (s *Server) handleHostQR(w http.ResponseWriter, r *http.Request) {
	want := r.URL.Query().Get("ip")
	code := s.sessions.Code()
	var link string
	for i, a := range s.addressViews(code) {
		if a.IP == want || (want == "" && i == 0) {
			link = a.URL
			break
		}
	}
	if link == "" {
		writeError(w, http.StatusNotFound, "address_not_found", "That address is not available on this computer.")
		return
	}
	svg, err := qr.SVG(link, "#04070d", "#e6f7ff")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not draw the QR code.")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(svg))
}

func (s *Server) handleHostSettings(w http.ResponseWriter, r *http.Request) {
	var body settingsView
	if !readJSON(w, r, &body) {
		return
	}
	v := settings.Values{
		TrayLimit: body.TrayLimit,
		FileTTL:   time.Duration(body.FileTTLSeconds) * time.Second,
	}
	if err := s.settings.Update(v); err != nil {
		if errors.Is(err, settings.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "invalid_settings", err.Error())
			return
		}
		s.log.Error("save settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Could not save the settings.")
		return
	}
	s.log.Info("settings changed", "trayLimit", v.TrayLimit, "fileTTL", v.FileTTL)
	s.store.Sweep() // a shorter expiry applies to files already in the tray
	s.Notify()
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleHostRotate(w http.ResponseWriter, _ *http.Request) {
	code := s.sessions.RotateCode()
	s.log.Info("join code rotated")
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

// handleHostEnd empties the tray and signs every device out, including the
// host's own browser, which is admitted again on its next request.
func (s *Server) handleHostEnd(w http.ResponseWriter, _ *http.Request) {
	s.store.Clear()
	code := s.sessions.End()
	s.log.Info("session ended by host")
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

func (s *Server) handleHostRemoveDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.sessions.Remove(id); err != nil {
		if errors.Is(err, session.ErrNotFound) {
			writeError(w, http.StatusNotFound, "device_not_found", "That device is not in the session.")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Could not remove the device.")
		return
	}
	s.store.TerminateUploadsOf(id)
	s.log.Info("device removed by host", "device", id)
	w.WriteHeader(http.StatusNoContent)
}
