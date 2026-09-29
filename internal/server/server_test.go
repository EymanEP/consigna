package server

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/EymanEP/consigna/internal/events"
	"github.com/EymanEP/consigna/internal/netinfo"
	"github.com/EymanEP/consigna/internal/session"
	"github.com/EymanEP/consigna/internal/settings"
	"github.com/EymanEP/consigna/internal/sizes"
	"github.com/EymanEP/consigna/internal/store"
)

type env struct {
	t        *testing.T
	srv      *Server
	ts       *httptest.Server
	sessions *session.Manager
	store    *store.Store
	settings *settings.Store
}

// newEnv starts a server. With trustLoopback false, test clients (which
// connect over loopback) behave like ordinary LAN devices.
func newEnv(t *testing.T, trustLoopback bool) *env {
	t.Helper()
	cfg, err := settings.New(settings.Values{TrayLimit: 100 * sizes.MB, FileTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	hub := &events.Hub{}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.New(store.Options{Dir: t.TempDir(), Limits: cfg, OnChange: hub.Notify, Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	sessions := session.New(session.Options{OnChange: hub.Notify})
	web := fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html><title>Consigna</title>")},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
		"favicon.svg":       {Data: []byte("<svg/>")},
	}
	srv, err := New(Options{
		Store: st, Sessions: sessions, Settings: cfg, Hub: hub, Web: web,
		Addresses: func() []netinfo.Address {
			return []netinfo.Address{{Interface: "wlan0", IP: netip.MustParseAddr("192.168.1.47"), Kind: netinfo.KindWiFi}}
		},
		Port: 7431, TrustLoopback: trustLoopback, Version: "test", Logger: quiet,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		srv.Stop()
		st.Close()
		ts.Close()
	})
	return &env{t: t, srv: srv, ts: ts, sessions: sessions, store: st, settings: cfg}
}

// client is a browser-like client with its own cookie jar.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: e.t, base: e.ts.URL, http: &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (c *client) do(method, path string, body io.Reader, headers map[string]string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", c.base)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (c *client) json(method, path string, in, out any) *http.Response {
	c.t.Helper()
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	resp := c.do(method, path, body, map[string]string{"Content-Type": "application/json"})
	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			c.t.Fatal(err)
		}
	}
	return resp
}

func (c *client) join(code string) {
	c.t.Helper()
	resp := c.do(http.MethodGet, "/t/"+strings.ToLower(code), nil, nil)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		c.t.Fatalf("join: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func (c *client) state() stateView {
	c.t.Helper()
	var st stateView
	if resp := c.json(http.MethodGet, "/api/v1/state", nil, &st); resp.StatusCode != http.StatusOK {
		c.t.Fatalf("state: %d", resp.StatusCode)
	}
	return st
}

// upload sends body with the tus protocol, in chunks of chunk bytes.
func (c *client) upload(name string, body []byte, chunk int) string {
	c.t.Helper()
	resp := c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{
		"Tus-Resumable":   "1.0.0",
		"Upload-Length":   strconv.Itoa(len(body)),
		"Upload-Metadata": "filename " + base64.StdEncoding.EncodeToString([]byte(name)) + ",filetype " + base64.StdEncoding.EncodeToString([]byte("text/plain")),
	})
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		c.t.Fatalf("create: %d %s", resp.StatusCode, b)
	}
	loc := resp.Header.Get("Location")
	for off := 0; off < len(body); off += chunk {
		end := min(off+chunk, len(body))
		resp := c.do(http.MethodPatch, loc, bytes.NewReader(body[off:end]), map[string]string{
			"Tus-Resumable": "1.0.0",
			"Upload-Offset": strconv.Itoa(off),
			"Content-Type":  "application/offset+octet-stream",
		})
		if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Upload-Offset") != strconv.Itoa(end) {
			c.t.Fatalf("patch at %d: %d offset=%s", off, resp.StatusCode, resp.Header.Get("Upload-Offset"))
		}
	}
	return loc
}

func TestUnjoinedDeviceIsRejected(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	for _, p := range []string{"/api/v1/state", "/api/v1/events", "/api/v1/files/x/content", "/api/v1/archive?ids=x"} {
		if resp := c.do(http.MethodGet, p, nil, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: %d", p, resp.StatusCode)
		}
	}
}

func TestJoinLink(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	resp := c.do(http.MethodGet, "/t/wrong1", nil, nil)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/?join=invalid_code" {
		t.Fatalf("wrong code: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	c.join(e.sessions.Code())
	st := c.state()
	if st.Me.Name == "" || !st.Me.IsMe || st.Me.IP != "" {
		t.Fatalf("me = %+v", st.Me)
	}
	if st.Host != nil {
		t.Fatal("guest received host details")
	}
	code := strings.ToLower(e.sessions.Code())
	// Seen through a loopback address, the link points at the LAN address.
	if st.Session.JoinURL != "http://192.168.1.47:7431/t/"+code {
		t.Fatalf("joinUrl = %s", st.Session.JoinURL)
	}
	// Otherwise it is the address the device itself is using.
	req, _ := http.NewRequest(http.MethodGet, e.ts.URL+"/api/v1/state", nil)
	req.Host = "10.1.2.3:7431"
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var viaLAN stateView
	if err := json.NewDecoder(resp.Body).Decode(&viaLAN); err != nil {
		t.Fatal(err)
	}
	if viaLAN.Session.JoinURL != "http://10.1.2.3:7431/t/"+code {
		t.Fatalf("joinUrl via LAN = %s", viaLAN.Session.JoinURL)
	}
	// Opening the link again does not create a second device.
	c.join(e.sessions.Code())
	if n := len(e.sessions.Devices()); n != 1 {
		t.Fatalf("devices = %d", n)
	}
}

func TestJoinWithCode(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	if resp := c.json(http.MethodPost, "/api/v1/join", map[string]string{"code": "nope"}, nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong code: %d", resp.StatusCode)
	}
	var st stateView
	if resp := c.json(http.MethodPost, "/api/v1/join", map[string]string{"code": " " + strings.ToLower(e.sessions.Code())}, &st); resp.StatusCode != http.StatusOK {
		t.Fatalf("join: %d", resp.StatusCode)
	}
	if st.Me.ID == "" {
		t.Fatal("no device in response")
	}
	c.state()
	if resp := c.json(http.MethodPost, "/api/v1/join", map[string]any{"code": "x", "extra": 1}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field: %d", resp.StatusCode)
	}
}

func TestUploadDownloadDelete(t *testing.T) {
	e := newEnv(t, false)
	alice, bob := e.client(), e.client()
	alice.join(e.sessions.Code())
	bob.join(e.sessions.Code())

	body := bytes.Repeat([]byte("consigna "), 5000)
	alice.upload("../notes 📄.txt", body, 7000)

	st := bob.state()
	if len(st.Files) != 1 {
		t.Fatalf("files = %d", len(st.Files))
	}
	f := st.Files[0]
	if f.Name != "notes 📄.txt" || f.Size != int64(len(body)) || f.UploaderName != alice.state().Me.Name {
		t.Fatalf("file = %+v", f)
	}
	if st.Storage.Used != int64(len(body)) {
		t.Fatalf("storage = %+v", st.Storage)
	}

	resp := bob.do(http.MethodGet, "/api/v1/files/"+f.ID+"/content", nil, nil)
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !bytes.Equal(got, body) {
		t.Fatalf("download: %d, %d bytes", resp.StatusCode, len(got))
	}
	h := resp.Header
	if h.Get("Content-Type") != "application/octet-stream" ||
		!strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") ||
		!strings.Contains(h.Get("Content-Disposition"), "filename*=UTF-8''notes%20%F0%9F%93%84.txt") ||
		!strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("download headers: %v", h)
	}

	resp = bob.do(http.MethodGet, "/api/v1/files/"+f.ID+"/content", nil, map[string]string{"Range": "bytes=9-17"})
	got, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPartialContent || string(got) != "consigna " {
		t.Fatalf("range: %d %q", resp.StatusCode, got)
	}

	if resp := bob.do(http.MethodDelete, "/api/v1/files/"+f.ID, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if resp := bob.do(http.MethodDelete, "/api/v1/files/"+f.ID, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete: %d", resp.StatusCode)
	}
	if n := len(alice.state().Files); n != 0 {
		t.Fatalf("files after delete = %d", n)
	}
}

func TestTusProtocol(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	c.join(e.sessions.Code())
	other := e.client()
	other.join(e.sessions.Code())

	resp := c.do(http.MethodOptions, "/api/v1/uploads/", nil, nil)
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Tus-Version") != "1.0.0" ||
		!strings.Contains(resp.Header.Get("Tus-Extension"), "termination") {
		t.Fatalf("options: %d %v", resp.StatusCode, resp.Header)
	}
	if resp := c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{"Tus-Resumable": "0.2.2", "Upload-Length": "1"}); resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("bad version: %d", resp.StatusCode)
	}
	if resp := c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{"Tus-Resumable": "1.0.0", "Upload-Defer-Length": "1"}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("defer length: %d", resp.StatusCode)
	}
	if resp := c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": "-5"}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative length: %d", resp.StatusCode)
	}
	if resp := c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": strconv.Itoa(200 * 1000 * 1000)}); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("over quota: %d", resp.StatusCode)
	}

	resp = c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": "10"})
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusCreated || !strings.HasPrefix(loc, "/api/v1/uploads/") {
		t.Fatalf("create: %d %s", resp.StatusCode, loc)
	}
	patch := func(cl *client, off string, body string, ct string) *http.Response {
		return cl.do(http.MethodPatch, loc, strings.NewReader(body), map[string]string{
			"Tus-Resumable": "1.0.0", "Upload-Offset": off, "Content-Type": ct,
		})
	}
	if resp := patch(c, "0", "abc", "text/plain"); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong content type: %d", resp.StatusCode)
	}
	if resp := patch(other, "0", "abc", tusContentType); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("other device patch: %d", resp.StatusCode)
	}
	if resp := patch(c, "0", "abcd", tusContentType); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("first chunk: %d", resp.StatusCode)
	}
	if resp := patch(c, "0", "abcd", tusContentType); resp.StatusCode != http.StatusConflict || resp.Header.Get("Upload-Offset") != "4" {
		t.Fatalf("stale offset: %d %s", resp.StatusCode, resp.Header.Get("Upload-Offset"))
	}
	resp = c.do(http.MethodHead, loc, nil, map[string]string{"Tus-Resumable": "1.0.0"})
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Upload-Offset") != "4" || resp.Header.Get("Upload-Length") != "10" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("head: %d %v", resp.StatusCode, resp.Header)
	}
	if resp := patch(c, "4", "efghij", tusContentType+"; charset=binary"); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("last chunk: %d", resp.StatusCode)
	}
	// A client that missed the final response learns the upload is done.
	resp = c.do(http.MethodHead, loc, nil, map[string]string{"Tus-Resumable": "1.0.0"})
	if resp.Header.Get("Upload-Offset") != "10" {
		t.Fatalf("head after completion: %v", resp.Header)
	}
	if n := len(c.state().Files); n != 1 {
		t.Fatalf("files = %d", n)
	}

	// Termination frees the reservation.
	resp = c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": "50"})
	loc2 := resp.Header.Get("Location")
	if got := c.state().Storage.Reserved; got != 50 {
		t.Fatalf("reserved = %d", got)
	}
	if resp := c.do(http.MethodDelete, loc2, nil, map[string]string{"Tus-Resumable": "1.0.0"}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("terminate: %d", resp.StatusCode)
	}
	if got := c.state().Storage.Reserved; got != 0 {
		t.Fatalf("reserved after terminate = %d", got)
	}
}

func TestZeroByteUpload(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	c.join(e.sessions.Code())
	resp := c.do(http.MethodPost, "/api/v1/uploads/", nil, map[string]string{
		"Tus-Resumable": "1.0.0", "Upload-Length": "0",
		"Upload-Metadata": "filename " + base64.StdEncoding.EncodeToString([]byte("empty.txt")),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	resp = c.do(http.MethodHead, resp.Header.Get("Location"), nil, map[string]string{"Tus-Resumable": "1.0.0"})
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Upload-Offset") != "0" {
		t.Fatalf("head: %d %v", resp.StatusCode, resp.Header)
	}
	if files := c.state().Files; len(files) != 1 || files[0].Name != "empty.txt" {
		t.Fatalf("files = %+v", files)
	}
}

func TestArchive(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	c.join(e.sessions.Code())
	c.upload("a.txt", []byte("alpha"), 100)
	c.upload("a.txt", []byte("second alpha"), 100)
	c.upload("b.bin", []byte("beta"), 100)
	st := c.state()
	ids := make([]string, 0, len(st.Files))
	for _, f := range st.Files {
		ids = append(ids, f.ID)
	}
	resp := c.do(http.MethodGet, "/api/v1/archive?ids="+strings.Join(ids, ","), nil, nil)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("archive: %d %v", resp.StatusCode, resp.Header)
	}
	data, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	if len(got) != 3 || got["b.bin"] != "beta" || got["a.txt"] == "" || got["a (2).txt"] == "" {
		t.Fatalf("zip contents = %v", got)
	}

	if resp := c.do(http.MethodGet, "/api/v1/archive?ids="+ids[0]+",missing", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing file: %d", resp.StatusCode)
	}
	if resp := c.do(http.MethodGet, "/api/v1/archive", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("no ids: %d", resp.StatusCode)
	}

	var out map[string]int
	if resp := c.json(http.MethodPost, "/api/v1/files/delete", map[string][]string{"ids": append(ids, "missing")}, &out); resp.StatusCode != http.StatusOK || out["deleted"] != 3 {
		t.Fatalf("bulk delete: %d %v", resp.StatusCode, out)
	}
}

func TestRenameAndLeave(t *testing.T) {
	e := newEnv(t, false)
	a, b := e.client(), e.client()
	a.join(e.sessions.Code())
	b.join(e.sessions.Code())
	var me deviceView
	if resp := a.json(http.MethodPatch, "/api/v1/me", map[string]string{"name": " Work laptop "}, &me); resp.StatusCode != http.StatusOK || me.Name != "Work laptop" {
		t.Fatalf("rename: %d %+v", resp.StatusCode, me)
	}
	if resp := b.json(http.MethodPatch, "/api/v1/me", map[string]string{"name": "work LAPTOP"}, nil); resp.StatusCode != http.StatusConflict {
		t.Fatalf("taken: %d", resp.StatusCode)
	}
	if resp := b.json(http.MethodPatch, "/api/v1/me", map[string]string{"name": ""}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty: %d", resp.StatusCode)
	}
	if resp := a.do(http.MethodPost, "/api/v1/leave", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("leave: %d", resp.StatusCode)
	}
	if resp := a.do(http.MethodGet, "/api/v1/state", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after leave: %d", resp.StatusCode)
	}
}

func TestSecurityChecks(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	c.join(e.sessions.Code())
	c.upload("a.txt", []byte("a"), 10)
	id := c.state().Files[0].ID

	req, _ := http.NewRequest(http.MethodDelete, e.ts.URL+"/api/v1/files/"+id, nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, _ := c.http.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin delete: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodDelete, e.ts.URL+"/api/v1/files/"+id, nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, _ = c.http.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site delete: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, e.ts.URL+"/api/v1/state", nil)
	req.Host = "attacker.example:7431"
	resp, _ = c.http.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Fatalf("rebinding host: %d", resp.StatusCode)
	}

	resp = c.do(http.MethodGet, "/", nil, nil)
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}

	// Host-only endpoints are refused to LAN devices.
	for _, p := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/host/rotate-code"},
		{http.MethodPost, "/api/v1/host/end-session"},
		{http.MethodPut, "/api/v1/host/settings"},
		{http.MethodGet, "/api/v1/host/qr.svg"},
		{http.MethodDelete, "/api/v1/host/devices/x"},
	} {
		if resp := c.do(p.method, p.path, strings.NewReader("{}"), nil); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s: %d", p.method, p.path, resp.StatusCode)
		}
	}
}

func TestProxiedLoopbackIsNotHost(t *testing.T) {
	e := newEnv(t, true)
	c := e.client()
	resp := c.do(http.MethodGet, "/api/v1/state", nil, map[string]string{"X-Forwarded-For": "192.168.1.9"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("proxied request admitted: %d", resp.StatusCode)
	}
}

func TestHost(t *testing.T) {
	e := newEnv(t, true)
	host := e.client()
	st := host.state() // auto-joined over loopback
	if !st.Me.IsHost || st.Host == nil || len(st.Host.Addresses) != 1 {
		t.Fatalf("host state = %+v", st)
	}
	code := strings.ToLower(e.sessions.Code())
	if st.Session.JoinURL != "http://192.168.1.47:7431/t/"+code {
		t.Fatalf("host join url = %s", st.Session.JoinURL)
	}
	if st.Me.IP == "" {
		t.Fatal("host view lacks device IPs")
	}

	resp := host.do(http.MethodGet, "/api/v1/host/qr.svg?ip=192.168.1.47", nil, nil)
	svg, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/svg+xml" || !bytes.HasPrefix(svg, []byte("<svg")) {
		t.Fatalf("qr: %d", resp.StatusCode)
	}
	if resp := host.do(http.MethodGet, "/api/v1/host/qr.svg?ip=8.8.8.8", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign ip qr: %d", resp.StatusCode)
	}

	var sv settingsView
	if resp := host.json(http.MethodPut, "/api/v1/host/settings", settingsView{TrayLimit: 5 * sizes.GB, FileTTLSeconds: 7200}, &sv); resp.StatusCode != http.StatusOK {
		t.Fatalf("settings: %d", resp.StatusCode)
	}
	if v := e.settings.Get(); v.TrayLimit != 5*sizes.GB || v.FileTTL != 2*time.Hour {
		t.Fatalf("settings not applied: %+v", v)
	}
	if resp := host.json(http.MethodPut, "/api/v1/host/settings", settingsView{TrayLimit: 1, FileTTLSeconds: 7200}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid settings: %d", resp.StatusCode)
	}

	old := e.sessions.Code()
	var rot map[string]string
	host.json(http.MethodPost, "/api/v1/host/rotate-code", nil, &rot)
	if rot["code"] == "" || rot["code"] != e.sessions.Code() {
		t.Fatalf("rotate: %v (old %s)", rot, old)
	}

	_, guest, _ := e.sessions.JoinTrusted("192.168.1.9", "Mozilla/5.0 (iPhone; Mobile)", false)
	if resp := host.do(http.MethodDelete, "/api/v1/host/devices/"+guest.ID, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("remove device: %d", resp.StatusCode)
	}
	if resp := host.do(http.MethodDelete, "/api/v1/host/devices/"+guest.ID, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("remove twice: %d", resp.StatusCode)
	}

	host.upload("a.txt", []byte("a"), 10)
	if resp := host.do(http.MethodPost, "/api/v1/host/end-session", nil, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("end: %d", resp.StatusCode)
	}
	st = host.state() // re-admitted automatically
	if len(st.Files) != 0 || len(st.Devices) != 1 {
		t.Fatalf("after end: %d files, %d devices", len(st.Files), len(st.Devices))
	}
}

func TestEventsStream(t *testing.T) {
	e := newEnv(t, false)
	a, b := e.client(), e.client()
	a.join(e.sessions.Code())
	b.join(e.sessions.Code())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.ts.URL+"/api/v1/events", nil)
	resp, err := a.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	events := make(chan [2]string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		var name string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				events <- [2]string{name, strings.TrimPrefix(line, "data: ")}
			}
		}
		close(events)
	}()
	next := func() (string, stateView) {
		t.Helper()
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("stream closed")
			}
			var st stateView
			_ = json.Unmarshal([]byte(ev[1]), &st)
			return ev[0], st
		case <-time.After(5 * time.Second):
			t.Fatal("no event")
		}
		return "", stateView{}
	}
	// Wait until a sees itself online, then react to b's upload.
	for {
		name, st := next()
		if name != "state" {
			t.Fatalf("event %q", name)
		}
		if st.Me.Online {
			break
		}
	}
	b.upload("hello.txt", []byte("hi"), 10)
	for {
		_, st := next()
		if len(st.Files) == 1 && st.Files[0].Name == "hello.txt" {
			break
		}
	}
}

func TestEventsEndedAndShutdown(t *testing.T) {
	e := newEnv(t, false)
	a := e.client()
	a.join(e.sessions.Code())
	me := a.state().Me

	read := func() string {
		req, _ := http.NewRequest(http.MethodGet, e.ts.URL+"/api/v1/events", nil)
		resp, err := a.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	done := make(chan string)
	go func() { done <- read() }()
	waitOnline(t, e, me.ID)
	e.srv.Stop()
	select {
	case out := <-done:
		if !strings.Contains(out, "event: shutdown") {
			t.Fatalf("no shutdown event in %q", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not end on shutdown")
	}
}

func TestEventsEndWhenDeviceRemoved(t *testing.T) {
	e := newEnv(t, false)
	a := e.client()
	a.join(e.sessions.Code())
	me := a.state().Me
	done := make(chan string)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, e.ts.URL+"/api/v1/events", nil)
		resp, err := a.http.Do(req)
		if err != nil {
			done <- err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		done <- string(b)
	}()
	waitOnline(t, e, me.ID)
	if err := e.sessions.Remove(me.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-done:
		if !strings.Contains(out, "event: ended") {
			t.Fatalf("no ended event in %q", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not end")
	}
}

func TestSPA(t *testing.T) {
	e := newEnv(t, false)
	c := e.client()
	for path, want := range map[string]int{"/": 200, "/host": 200, "/assets/app-abc.js": 200, "/favicon.svg": 200, "/missing.js": 404} {
		if resp := c.do(http.MethodGet, path, nil, nil); resp.StatusCode != want {
			t.Errorf("%s: %d, want %d", path, resp.StatusCode, want)
		}
	}
	if resp := c.do(http.MethodGet, "/assets/app-abc.js", nil, nil); !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Error("hashed asset not cached")
	}
	if resp := c.do(http.MethodGet, "/host", nil, nil); resp.Header.Get("Cache-Control") != "no-cache" {
		t.Error("app shell cached")
	}
	if resp := c.do(http.MethodGet, "/api/v1/nope", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown api: %d", resp.StatusCode)
	}
	if resp := c.do(http.MethodGet, "/healthz", nil, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: %d", resp.StatusCode)
	}
}

func TestDeviceKind(t *testing.T) {
	cases := map[string]string{
		"": "unknown",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mobile/15E148":         "phone",
		"Mozilla/5.0 (Linux; Android 15; Pixel 9) Chrome/130 Mobile Safari/537.36":     "phone",
		"Mozilla/5.0 (Linux; Android 14; SM-X700) Chrome/130 Safari/537.36":            "tablet",
		"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)":                                "tablet",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 Safari/605": "desktop",
	}
	for ua, want := range cases {
		if got := deviceKind(ua); got != want {
			t.Errorf("deviceKind(%q) = %s, want %s", ua, got, want)
		}
	}
}

func TestContentDisposition(t *testing.T) {
	got := contentDisposition(`a "quoted" 100%\.txt`)
	want := `attachment; filename="a _quoted_ 100__.txt"; filename*=UTF-8''a%20%22quoted%22%20100%25%5C.txt`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func waitOnline(t *testing.T, e *env, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if d, ok := e.sessions.Get(id); ok && d.Online {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("device never came online")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
