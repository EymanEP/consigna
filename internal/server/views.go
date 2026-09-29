package server

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/EymanEP/consigna/internal/session"
)

// The types below are the JSON contract with the web UI. Keep them in sync
// with web/src/lib/types.ts.

type stateView struct {
	Me         deviceView   `json:"me"`
	Session    sessionView  `json:"session"`
	Devices    []deviceView `json:"devices"`
	Files      []fileView   `json:"files"`
	Storage    storageView  `json:"storage"`
	Settings   settingsView `json:"settings"`
	ServerTime time.Time    `json:"serverTime"`
	Host       *hostView    `json:"host,omitempty"`
}

type sessionView struct {
	Code    string `json:"code"`
	JoinURL string `json:"joinUrl"`
}

type deviceView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Online bool   `json:"online"`
	IsHost bool   `json:"isHost"`
	IsMe   bool   `json:"isMe"`
	// Only sent to the host.
	IP        string     `json:"ip,omitempty"`
	UserAgent string     `json:"userAgent,omitempty"`
	JoinedAt  *time.Time `json:"joinedAt,omitempty"`
	LastSeen  *time.Time `json:"lastSeen,omitempty"`
}

type fileView struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	UploaderID   string    `json:"uploaderId"`
	UploaderName string    `json:"uploaderName"`
	AddedAt      time.Time `json:"addedAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type storageView struct {
	Used     int64 `json:"used"`
	Reserved int64 `json:"reserved"`
	Limit    int64 `json:"limit"`
}

type settingsView struct {
	TrayLimit      int64 `json:"trayLimit"`
	FileTTLSeconds int64 `json:"fileTtlSeconds"`
}

type hostView struct {
	Version   string        `json:"version"`
	Addresses []addressView `json:"addresses"`
	Uploads   []uploadView  `json:"uploads"`
}

type addressView struct {
	Interface string `json:"interface"`
	IP        string `json:"ip"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
}

type uploadView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Offset    int64     `json:"offset"`
	OwnerID   string    `json:"ownerId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// buildState renders the tray as seen by device me.
func (s *Server) buildState(r *http.Request, me session.Device) stateView {
	isHost := s.isLoopback(r)
	devices := s.sessions.Devices()
	names := make(map[string]string, len(devices))
	st := stateView{
		Devices:    make([]deviceView, 0, len(devices)),
		ServerTime: time.Now().UTC(),
	}
	for _, d := range devices {
		names[d.ID] = d.Name
		v := s.deviceView(d, me.ID, isHost)
		if d.ID == me.ID {
			st.Me = v
		}
		st.Devices = append(st.Devices, v)
	}
	if st.Me.ID == "" {
		st.Me = s.deviceView(me, me.ID, isHost)
	}

	files := s.store.Files()
	st.Files = make([]fileView, 0, len(files))
	for _, f := range files {
		name := f.UploaderName
		if current, ok := names[f.UploaderID]; ok {
			name = current
		}
		st.Files = append(st.Files, fileView{
			ID: f.ID, Name: f.Name, Size: f.Size,
			UploaderID: f.UploaderID, UploaderName: name,
			AddedAt: f.AddedAt.UTC(), ExpiresAt: f.ExpiresAt.UTC(),
		})
	}

	u := s.store.Usage()
	st.Storage = storageView{Used: u.Used, Reserved: u.Reserved, Limit: u.Limit}
	v := s.settings.Get()
	st.Settings = settingsView{TrayLimit: v.TrayLimit, FileTTLSeconds: int64(v.FileTTL / time.Second)}

	code := s.sessions.Code()
	addrs := s.addressViews(code)
	st.Session = sessionView{Code: code, JoinURL: s.joinURL(r, code, addrs)}

	if isHost {
		h := &hostView{Version: s.version, Addresses: addrs, Uploads: []uploadView{}}
		for _, up := range s.store.Uploads() {
			h.Uploads = append(h.Uploads, uploadView{
				ID: up.ID, Name: up.Name, Size: up.Size, Offset: up.Offset,
				OwnerID: up.OwnerID, UpdatedAt: up.UpdatedAt.UTC(),
			})
		}
		st.Host = h
	}
	return st
}

func (s *Server) deviceView(d session.Device, meID string, detailed bool) deviceView {
	v := deviceView{
		ID: d.ID, Name: d.Name, Kind: deviceKind(d.UserAgent),
		Online: d.Online, IsHost: d.IsHost, IsMe: d.ID == meID,
	}
	if detailed {
		joined, seen := d.JoinedAt.UTC(), d.LastSeen.UTC()
		v.IP, v.UserAgent, v.JoinedAt, v.LastSeen = d.IP, d.UserAgent, &joined, &seen
	}
	return v
}

func (s *Server) addressViews(code string) []addressView {
	addrs := s.addresses()
	out := make([]addressView, 0, len(addrs))
	for _, a := range addrs {
		host := net.JoinHostPort(a.IP.String(), fmt.Sprint(s.port))
		out = append(out, addressView{
			Interface: a.Interface, IP: a.IP.String(), Kind: string(a.Kind),
			URL: joinLink("http", host, code),
		})
	}
	return out
}

// joinURL is the link a device should share: the address it is using itself,
// unless that is a loopback name, which is useless to anyone else.
func (s *Server) joinURL(r *http.Request, code string, addrs []addressView) string {
	host := r.Host
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	ip := net.ParseIP(strings.Trim(name, "[]"))
	loop := strings.EqualFold(name, "localhost") || (ip != nil && ip.IsLoopback())
	if loop && len(addrs) > 0 {
		return addrs[0].URL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return joinLink(scheme, host, code)
}

func joinLink(scheme, host, code string) string {
	return scheme + "://" + host + "/t/" + strings.ToLower(code)
}

var (
	tabletUA  = regexp.MustCompile(`(?i)ipad|tablet|kindle|silk`)
	androidUA = regexp.MustCompile(`(?i)android`)
	mobileUA  = regexp.MustCompile(`(?i)iphone|ipod|windows phone|mobile`)
)

// deviceKind guesses the device type from its user agent, for the icon only.
func deviceKind(ua string) string {
	switch {
	case ua == "":
		return "unknown"
	case tabletUA.MatchString(ua):
		return "tablet"
	case mobileUA.MatchString(ua):
		return "phone"
	case androidUA.MatchString(ua):
		return "tablet" // Android without "Mobile" is a tablet
	default:
		return "desktop"
	}
}
