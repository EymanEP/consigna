package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/EymanEP/consigna/internal/session"
)

// cookieName holds the device's secret token.
const cookieName = "consigna"

// cookieMaxAge keeps a device signed in across browser restarts for as long
// as the server keeps running.
const cookieMaxAge = 30 * 24 * time.Hour

type ctxKey int

const deviceKey ctxKey = iota

// deviceFrom returns the authenticated device stored by requireDevice.
func deviceFrom(r *http.Request) session.Device {
	d, _ := r.Context().Value(deviceKey).(session.Device)
	return d
}

// authenticate resolves the request's cookie to a device. The host's own
// browser (loopback) is admitted automatically.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (session.Device, bool) {
	if c, err := r.Cookie(cookieName); err == nil {
		if d, ok := s.sessions.Authenticate(c.Value, clientIP(r), r.UserAgent()); ok {
			return d, true
		}
	}
	if s.isLoopback(r) {
		token, d, err := s.sessions.JoinTrusted(clientIP(r), r.UserAgent(), true)
		if err == nil {
			s.setCookie(w, r, token)
			s.log.Info("host browser joined", "device", d.Name)
			return d, true
		}
	}
	return session.Device{}, false
}

// requireDevice rejects requests without a valid device.
func (s *Server) requireDevice(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, ok := s.authenticate(w, r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "not_joined",
				"This device is not in the session. Scan the QR code or open the link from the host.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), deviceKey, d)))
	}
}

// requireHost only admits the host machine itself.
func (s *Server) requireHost(next http.HandlerFunc) http.HandlerFunc {
	return s.requireDevice(func(w http.ResponseWriter, r *http.Request) {
		if !s.isLoopback(r) {
			writeError(w, http.StatusForbidden, "host_only", "Only the host computer can do this.")
			return
		}
		next(w, r)
	})
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(cookieMaxAge / time.Second),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
}

// joinError maps session errors to HTTP responses.
func joinError(err error) (int, string, string) {
	switch {
	case errors.Is(err, session.ErrInvalidCode):
		return http.StatusForbidden, "invalid_code", "That code does not match this session."
	case errors.Is(err, session.ErrRateLimited):
		return http.StatusTooManyRequests, "rate_limited", "Too many wrong codes. Wait a few minutes and try again."
	case errors.Is(err, session.ErrFull):
		return http.StatusServiceUnavailable, "session_full", "This session has reached its device limit."
	default:
		return http.StatusInternalServerError, "internal", "Something went wrong."
	}
}
