// Package app wires Consigna's parts together and runs the server.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/EymanEP/consigna/internal/events"
	"github.com/EymanEP/consigna/internal/netinfo"
	"github.com/EymanEP/consigna/internal/server"
	"github.com/EymanEP/consigna/internal/session"
	"github.com/EymanEP/consigna/internal/settings"
	"github.com/EymanEP/consigna/internal/store"
)

// Config is everything Run needs.
type Config struct {
	// Bind is the IP address to listen on; empty or 0.0.0.0 means all.
	Bind string
	Port int
	// DataDir is the parent directory for tray contents.
	DataDir string
	// Settings holds the adjustable limits (already loaded and overridden).
	Settings *settings.Store
	// SettingsPath is shown in the banner; empty when not persisted.
	SettingsPath  string
	TrustLoopback bool
	AllowHosts    []string
	ShowQR        bool
	Version       string
	Web           fs.FS
	Logger        *slog.Logger
	// Out receives the human-readable banner.
	Out io.Writer
	// Ready, if set, is called with the listening address once serving.
	Ready func(addr net.Addr)
}

const (
	sweepEvery      = 30 * time.Second
	shutdownTimeout = 10 * time.Second
)

// Run serves until ctx is cancelled, then shuts down gracefully and deletes
// every file in the tray.
func Run(ctx context.Context, cfg Config) error {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Settings == nil {
		return errors.New("app: Settings is required")
	}

	dir, err := newRunDir(cfg.DataDir, log)
	if err != nil {
		return err
	}
	removed := false
	defer func() {
		if !removed {
			_ = dir.remove()
		}
	}()

	hub := &events.Hub{}
	st, err := store.New(store.Options{
		Dir:      dir.path,
		Limits:   cfg.Settings,
		OnChange: hub.Notify,
		Logger:   log,
	})
	if err != nil {
		return err
	}
	sessions := session.New(session.Options{OnChange: hub.Notify})

	ln, err := listen(ctx, cfg.Bind, cfg.Port)
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	addresses := addressLister(cfg.Bind, log)

	srv, err := server.New(server.Options{
		Store:         st,
		Sessions:      sessions,
		Settings:      cfg.Settings,
		Hub:           hub,
		Web:           cfg.Web,
		Addresses:     addresses,
		Port:          port,
		TrustLoopback: cfg.TrustLoopback,
		AllowedHosts:  cfg.AllowHosts,
		Version:       cfg.Version,
		Logger:        log,
	})
	if err != nil {
		_ = ln.Close()
		return err
	}

	httpSrv := &http.Server{
		Handler: srv,
		// No overall read/write timeouts: uploads and downloads may take
		// hours. Stalled transfers are cut by per-read/write idle deadlines.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelDebug),
	}

	printBanner(cfg.Out, bannerInfo{
		version:   cfg.Version,
		code:      sessions.Code(),
		port:      port,
		bind:      cfg.Bind,
		addresses: addresses(),
		settings:  cfg.Settings.Get(),
		settingsP: cfg.SettingsPath,
		dataDir:   dir.path,
		showQR:    cfg.ShowQR,
	})

	runCtx, stopBackground := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); st.Run(runCtx, sweepEvery) }()
	go func() {
		defer wg.Done()
		t := time.NewTicker(heartbeatEvery)
		defer t.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-t.C:
				if err := dir.touch(); err != nil {
					log.Warn("heartbeat", "err", err)
				}
			}
		}
	}()

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()
	if cfg.Ready != nil {
		cfg.Ready(ln.Addr())
	}
	log.Info("serving", "addr", ln.Addr().String(), "version", cfg.Version)

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err = <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}

	srv.Stop()
	st.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if shutdownErr := httpSrv.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Warn("forcing shutdown", "err", shutdownErr)
		_ = httpSrv.Close()
	}
	stopBackground()
	wg.Wait()
	removed = true
	if rmErr := dir.remove(); rmErr != nil {
		fmt.Fprintf(cfg.Out, "\n  Consigna stopped, but some files could not be deleted from %s.\n"+
			"  They will be removed the next time Consigna starts.\n", dir.path)
	} else {
		fmt.Fprintln(cfg.Out, "\n  Consigna stopped. Every file in the tray was deleted.")
	}
	return err
}

func listen(ctx context.Context, bind string, port int) (net.Listener, error) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return nil, fmt.Errorf("port %d is already in use; is Consigna already running? Try --port %d", port, port+1)
		}
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return ln, nil
}

// addressLister returns the reachable addresses, cached briefly because
// every live update asks for them.
func addressLister(bind string, log *slog.Logger) func() []netinfo.Address {
	if ip, err := netip.ParseAddr(bind); err == nil && !ip.IsUnspecified() {
		fixed := []netinfo.Address{{Interface: "bound", IP: ip.Unmap(), Kind: netinfo.KindOther}}
		if ip.IsLoopback() {
			fixed = nil
		}
		return func() []netinfo.Address { return fixed }
	}
	var (
		mu      sync.Mutex
		cached  []netinfo.Address
		fetched time.Time
	)
	return func() []netinfo.Address {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(fetched) < 5*time.Second {
			return cached
		}
		addrs, err := netinfo.Addresses()
		if err != nil {
			log.Warn("list network interfaces", "err", err)
		}
		cached, fetched = addrs, time.Now()
		return cached
	}
}
