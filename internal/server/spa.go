package server

import (
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

const notBuiltPage = `<!doctype html><meta charset="utf-8"><title>Consigna</title>
<body style="font-family:system-ui;background:#04070d;color:#cfeaff;padding:2rem">
<h1>Consigna is running</h1><p>The web UI was not built into this binary.
Run <code>make build</code> (or <code>npm --prefix web run build</code>) and rebuild.</p>`

// spa serves the built web UI. Hashed assets are cached forever; everything
// else, including the HTML shell for client-side routes, is revalidated.
func (s *Server) spa() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
			return
		}
		if s.web == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, notBuiltPage)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if s.serveAsset(w, r, name) {
			return
		}
		// Client-side routes (no file extension) get the app shell.
		if path.Ext(name) == "" && s.serveAsset(w, r, "index.html") {
			return
		}
		http.NotFound(w, r)
	})
}

func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request, name string) bool {
	f, err := s.web.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, time.Time{}, rs)
	return true
}
