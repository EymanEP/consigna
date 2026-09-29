package server

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EymanEP/consigna/internal/session"
	"github.com/EymanEP/consigna/internal/store"
)

// maxArchiveFiles bounds how many files one ZIP may bundle.
const maxArchiveFiles = 1000

// handleJoinLink admits a device that opened a join link (/t/{code}) and
// sends it to the app. Errors are reported to the app through the query
// string so it can explain them.
func (s *Server) handleJoinLink(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if _, ok := s.sessions.Authenticate(c.Value, clientIP(r), r.UserAgent()); ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	token, d, err := s.sessions.Join(r.PathValue("code"), clientIP(r), r.UserAgent())
	if err != nil {
		_, code, _ := joinError(err)
		s.log.Info("join refused", "remote", clientIP(r), "reason", code)
		http.Redirect(w, r, "/?join="+code, http.StatusSeeOther)
		return
	}
	s.setCookie(w, r, token)
	s.log.Info("device joined", "device", d.Name, "remote", clientIP(r))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleJoin admits a device that typed the session code.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	token, d, err := s.sessions.Join(body.Code, clientIP(r), r.UserAgent())
	if err != nil {
		status, code, msg := joinError(err)
		writeError(w, status, code, msg)
		return
	}
	s.setCookie(w, r, token)
	s.log.Info("device joined", "device", d.Name, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, s.buildState(r, d))
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.buildState(r, deviceFrom(r)))
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	d, err := s.sessions.Rename(deviceFrom(r).ID, body.Name)
	switch {
	case errors.Is(err, session.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "invalid_name",
			fmt.Sprintf("Use 1 to %d characters.", session.MaxNameRunes))
	case errors.Is(err, session.ErrNameTaken):
		writeError(w, http.StatusConflict, "name_taken", "Another device already uses that name.")
	case err != nil:
		writeError(w, http.StatusUnauthorized, "not_joined", "This device is no longer in the session.")
	default:
		writeJSON(w, http.StatusOK, s.deviceView(d, d.ID, s.isLoopback(r)))
	}
}

func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	d := deviceFrom(r)
	s.store.TerminateUploadsOf(d.ID)
	_ = s.sessions.Remove(d.ID)
	s.clearCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// setAttachment makes the browser save the response instead of rendering it.
// Uploaded files are untrusted: served inline, an HTML or SVG file would run
// scripts with this app's privileges. The sandbox CSP is a second fence.
func setAttachment(w http.ResponseWriter, name, contentType string) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", contentDisposition(name))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	rd, f, err := s.store.Open(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "file_not_found", "That file is no longer in the tray.")
		return
	}
	defer func() { _ = rd.Close() }()
	setAttachment(w, f.Name, "application/octet-stream")
	w.Header().Set("ETag", `"`+f.ID+`"`)
	rc := http.NewResponseController(w)
	http.ServeContent(idleWriter{ResponseWriter: w, rc: rc, timeout: idleTimeout}, r, "", f.AddedAt, rd)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if s.store.Delete(r.PathValue("id")) == 0 {
		writeError(w, http.StatusNotFound, "file_not_found", "That file is no longer in the tray.")
		return
	}
	s.log.Info("file deleted", "file", r.PathValue("id"), "by", deviceFrom(r).Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteFiles(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > maxArchiveFiles {
		writeError(w, http.StatusBadRequest, "invalid_ids", "Select between 1 and 1000 files.")
		return
	}
	n := s.store.Delete(body.IDs...)
	s.log.Info("files deleted", "count", n, "by", deviceFrom(r).Name)
	writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
}

// handleArchive streams several files as one ZIP. Nothing is written to disk
// and the download starts right away. Files are stored uncompressed: most
// large files (video, photos, archives) are already compressed, and the LAN
// is faster than the CPU would be.
func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	ids := parseIDs(r.URL.Query()["ids"])
	if len(ids) == 0 || len(ids) > maxArchiveFiles {
		writeError(w, http.StatusBadRequest, "invalid_ids", "Select between 1 and 1000 files.")
		return
	}
	type entry struct {
		rd   *store.Reader
		file store.File
	}
	entries := make([]entry, 0, len(ids))
	defer func() {
		for _, e := range entries {
			_ = e.rd.Close()
		}
	}()
	for _, id := range ids {
		rd, f, err := s.store.Open(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "file_not_found",
				"Some of the selected files are no longer in the tray.")
			return
		}
		entries = append(entries, entry{rd, f})
	}

	name := "consigna-" + time.Now().Format("2006-01-02-1504") + ".zip"
	setAttachment(w, name, "application/zip")
	rc := http.NewResponseController(w)
	zw := zip.NewWriter(idleWriter{ResponseWriter: w, rc: rc, timeout: idleTimeout})
	used := map[string]bool{}
	for _, e := range entries {
		hdr := &zip.FileHeader{
			Name:     uniqueName(e.file.Name, used),
			Method:   zip.Store,
			Modified: e.file.AddedAt,
		}
		hdr.SetMode(0o644)
		fw, err := zw.CreateHeader(hdr)
		if err == nil {
			_, err = io.Copy(fw, e.rd)
		}
		if err != nil {
			// Headers are already sent; abort the connection so the
			// browser reports a failed download instead of a corrupt ZIP.
			s.log.Info("archive aborted", "err", err)
			panic(http.ErrAbortHandler)
		}
	}
	if err := zw.Close(); err != nil {
		s.log.Info("archive aborted", "err", err)
		panic(http.ErrAbortHandler)
	}
}

// parseIDs accepts ids=a,b&ids=c, dropping blanks and duplicates.
func parseIDs(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		for _, id := range strings.Split(v, ",") {
			if id = strings.TrimSpace(id); id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// uniqueName returns name, or "name (2).ext" etc. if already used in a ZIP.
func uniqueName(name string, used map[string]bool) string {
	candidate := name
	stem, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		stem, ext = name[:i], name[i:]
	}
	for n := 2; used[strings.ToLower(candidate)]; n++ {
		candidate = stem + " (" + strconv.Itoa(n) + ")" + ext
	}
	used[strings.ToLower(candidate)] = true
	return candidate
}

// contentDisposition builds an attachment header that survives any file
// name: an ASCII fallback plus the exact UTF-8 name (RFC 6266 / RFC 8187).
func contentDisposition(name string) string {
	fallback := make([]rune, 0, len(name))
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == '%' {
			r = '_'
		}
		fallback = append(fallback, r)
	}
	v := mime.FormatMediaType("attachment", map[string]string{"filename": string(fallback)})
	if v == "" {
		v = "attachment"
	}
	return v + "; filename*=UTF-8''" + encodeRFC8187(name)
}

func encodeRFC8187(s string) string {
	const attrChars = "!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte(attrChars, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
