package server

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

// contentSecurityPolicy locks the app down to its own origin. React applies
// inline styles through the CSSOM, which CSP does not restrict, so no
// 'unsafe-inline' is needed.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; media-src 'self' blob:; " +
	"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// statusRecorder captures the status code for logging. Unwrap lets
// http.ResponseController reach the underlying writer for flushing and
// deadlines.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// logRequests logs each request once it completes and recovers from panics
// so a bug in one handler cannot take the server down.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				s.log.Error("panic serving request", "method", r.Method, "path", r.URL.Path,
					"panic", v, "stack", string(debug.Stack()))
				if rec.status == 0 {
					writeError(rec, http.StatusInternalServerError, "internal", "Something went wrong.")
				}
			}
			level := slog.LevelDebug
			if rec.status >= 500 {
				level = slog.LevelWarn
			}
			s.log.Log(r.Context(), level, "request",
				"method", r.Method, "path", r.URL.Path, "status", rec.status,
				"bytes", rec.bytes, "duration", time.Since(start).Round(time.Millisecond),
				"remote", clientIP(r))
		}()
		next.ServeHTTP(rec, r)
	})
}

// secureHeaders sets headers that apply to every response.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}

// checkHost rejects requests whose Host header is not an IP address,
// localhost or an explicitly allowed name. This defeats DNS-rebinding
// attacks, where a malicious website points its own domain at this machine.
func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			writeError(w, http.StatusMisdirectedRequest, "host_not_allowed",
				"This address is not allowed. Use the IP address shown by Consigna, or start it with --allow-host.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) hostAllowed(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	return s.allowedHosts[host]
}

// checkOrigin blocks cross-site requests that change state. Browsers always
// send Origin or Sec-Fetch-Site on such requests; clients that send neither
// are not browsers and cannot be tricked into acting for a user.
func checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				writeError(w, http.StatusForbidden, "cross_origin", "Cross-origin request refused.")
				return
			}
		} else if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			writeError(w, http.StatusForbidden, "cross_origin", "Cross-origin request refused.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the address the request came from. Forwarding headers are
// deliberately ignored: they are trivially spoofed on a LAN.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isLoopback reports whether the request comes from this machine itself and
// was not relayed by a proxy running on it.
func (s *Server) isLoopback(r *http.Request) bool {
	if !s.trustLoopback {
		return false
	}
	for _, h := range []string{"X-Forwarded-For", "Forwarded", "X-Real-Ip", "X-Forwarded-Host"} {
		if r.Header.Get(h) != "" {
			return false
		}
	}
	ip := net.ParseIP(clientIP(r))
	return ip != nil && ip.IsLoopback()
}
