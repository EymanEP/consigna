// Package server exposes the tray over HTTP: the JSON API, resumable uploads
// (tus 1.0), downloads, live updates (Server-Sent Events) and the web UI.
package server

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/EymanEP/consigna/internal/events"
	"github.com/EymanEP/consigna/internal/netinfo"
	"github.com/EymanEP/consigna/internal/session"
	"github.com/EymanEP/consigna/internal/settings"
	"github.com/EymanEP/consigna/internal/store"
)

// Options wire a Server to the rest of the application.
type Options struct {
	Store    *store.Store
	Sessions *session.Manager
	Settings *settings.Store
	Hub      *events.Hub
	// Web holds the built UI (index.html at its root). Nil serves a notice.
	Web fs.FS
	// Addresses lists the LAN addresses of this machine, best first.
	Addresses func() []netinfo.Address
	// Port is the TCP port the server listens on, used to build join links.
	Port int
	// TrustLoopback admits requests from this machine as the host.
	TrustLoopback bool
	// AllowedHosts are extra host names (besides IPs and localhost) that
	// may appear in the Host header.
	AllowedHosts []string
	Version      string
	Logger       *slog.Logger
}

// Server is an http.Handler.
type Server struct {
	store         *store.Store
	sessions      *session.Manager
	settings      *settings.Store
	hub           *events.Hub
	addresses     func() []netinfo.Address
	port          int
	trustLoopback bool
	allowedHosts  map[string]bool
	version       string
	log           *slog.Logger
	web           fs.FS

	handler  http.Handler
	stopOnce sync.Once
	stopping chan struct{}
}

// New builds the server and its routes.
func New(o Options) (*Server, error) {
	if o.Store == nil || o.Sessions == nil || o.Settings == nil || o.Hub == nil {
		return nil, errors.New("server: Store, Sessions, Settings and Hub are required")
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Addresses == nil {
		o.Addresses = func() []netinfo.Address { return nil }
	}
	s := &Server{
		store:         o.Store,
		sessions:      o.Sessions,
		settings:      o.Settings,
		hub:           o.Hub,
		addresses:     o.Addresses,
		port:          o.Port,
		trustLoopback: o.TrustLoopback,
		allowedHosts:  map[string]bool{},
		version:       o.Version,
		log:           o.Logger,
		web:           o.Web,
		stopping:      make(chan struct{}),
	}
	for _, h := range o.AllowedHosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			s.allowedHosts[h] = true
		}
	}
	if name, err := os.Hostname(); err == nil && name != "" {
		name = strings.ToLower(name)
		s.allowedHosts[name] = true
		s.allowedHosts[strings.TrimSuffix(name, ".local")+".local"] = true
	}
	s.handler = s.routes()
	return s, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /t/{code}", s.handleJoinLink)

	api := http.NewServeMux()
	api.HandleFunc("POST /api/v1/join", s.handleJoin)
	api.HandleFunc("GET /api/v1/state", s.requireDevice(s.handleState))
	api.HandleFunc("GET /api/v1/events", s.requireDevice(s.handleEvents))
	api.HandleFunc("PATCH /api/v1/me", s.requireDevice(s.handleRename))
	api.HandleFunc("POST /api/v1/leave", s.requireDevice(s.handleLeave))
	api.HandleFunc("GET /api/v1/files/{id}/content", s.requireDevice(s.handleDownload))
	api.HandleFunc("DELETE /api/v1/files/{id}", s.requireDevice(s.handleDeleteFile))
	api.HandleFunc("POST /api/v1/files/delete", s.requireDevice(s.handleDeleteFiles))
	api.HandleFunc("GET /api/v1/archive", s.requireDevice(s.handleArchive))

	api.HandleFunc("OPTIONS /api/v1/uploads/", s.handleTusOptions)
	api.HandleFunc("POST /api/v1/uploads/", s.requireDevice(s.handleTusCreate))
	api.HandleFunc("OPTIONS /api/v1/uploads/{id}", s.handleTusOptions)
	api.HandleFunc("HEAD /api/v1/uploads/{id}", s.requireDevice(s.handleTusHead))
	api.HandleFunc("PATCH /api/v1/uploads/{id}", s.requireDevice(s.handleTusPatch))
	api.HandleFunc("DELETE /api/v1/uploads/{id}", s.requireDevice(s.handleTusDelete))

	api.HandleFunc("GET /api/v1/host/qr.svg", s.requireHost(s.handleHostQR))
	api.HandleFunc("PUT /api/v1/host/settings", s.requireHost(s.handleHostSettings))
	api.HandleFunc("POST /api/v1/host/rotate-code", s.requireHost(s.handleHostRotate))
	api.HandleFunc("POST /api/v1/host/end-session", s.requireHost(s.handleHostEnd))
	api.HandleFunc("DELETE /api/v1/host/devices/{id}", s.requireHost(s.handleHostRemoveDevice))

	api.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "No such endpoint.")
	})
	mux.Handle("/api/", checkOrigin(api))
	mux.Handle("/", s.spa())

	return s.logRequests(secureHeaders(s.checkHost(mux)))
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// Stop tells long-lived connections (live updates) to finish so a graceful
// shutdown does not wait for them.
func (s *Server) Stop() {
	s.stopOnce.Do(func() { close(s.stopping) })
}

// Notify wakes every live connection so it re-sends the current state.
func (s *Server) Notify() { s.hub.Notify() }

// idleTimeout is how long an upload or download may make no progress before
// the connection is considered dead.
const idleTimeout = 60 * time.Second
