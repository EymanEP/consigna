package app

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/EymanEP/consigna/internal/netinfo"
	"github.com/EymanEP/consigna/internal/qr"
	"github.com/EymanEP/consigna/internal/settings"
	"github.com/EymanEP/consigna/internal/sizes"
)

type bannerInfo struct {
	version   string
	code      string
	port      int
	bind      string
	addresses []netinfo.Address
	settings  settings.Values
	settingsP string
	dataDir   string
	showQR    bool
}

func printBanner(w io.Writer, b bannerInfo) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
	p("")
	p("  ▌ CONSIGNA  %s", b.version)
	p("    LOCAL SESSION // NO INTERNET // %s", b.code)
	p("")
	if len(b.addresses) == 0 {
		if b.bind == "127.0.0.1" || b.bind == "::1" || b.bind == "localhost" {
			p("  Listening on this computer only (--bind %s).", b.bind)
		} else {
			p("  No network connection found. Connect to Wi-Fi or Ethernet;")
			p("  the host view will show the address once one is available.")
		}
	} else {
		best := b.addresses[0]
		link := "http://" + net.JoinHostPort(best.IP.String(), strconv.Itoa(b.port)) + "/t/" + strings.ToLower(b.code)
		p("  Open on your other devices (same Wi-Fi):")
		p("    %s   (%s)", link, best.Interface)
		if b.showQR {
			if code, err := qr.Terminal(link); err == nil {
				p("")
				fmt.Fprint(w, code)
			}
		}
		if len(b.addresses) > 1 {
			p("")
			p("  Other addresses of this computer:")
			for _, a := range b.addresses[1:] {
				p("    http://%s   (%s, %s)", net.JoinHostPort(a.IP.String(), strconv.Itoa(b.port)), a.Interface, a.Kind)
			}
		}
	}
	p("")
	p("  Host view (this computer only): http://localhost:%d/host", b.port)
	p("  Tray limit %s · files expire after %s", sizes.Format(b.settings.TrayLimit), humanDuration(b.settings.FileTTL))
	if b.settingsP != "" {
		p("  Settings file: %s", b.settingsP)
	}
	p("  Files are kept in %s", b.dataDir)
	p("")
	p("  Press Ctrl+C to stop. Every file is deleted when Consigna stops.")
	p("")
}

func humanDuration(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0 && d > 24*time.Hour:
		return fmt.Sprintf("%d days", int(d/(24*time.Hour)))
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	default:
		return d.String()
	}
}
