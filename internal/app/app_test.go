package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EymanEP/consigna/internal/settings"
)

func TestRunServesAndCleansUp(t *testing.T) {
	base := t.TempDir()
	// A leftover from a crashed run (stale heartbeat) is removed at start.
	stale := filepath.Join(base, "run-crashed")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	_ = os.WriteFile(filepath.Join(stale, aliveFile), nil, 0o600)
	_ = os.Chtimes(filepath.Join(stale, aliveFile), old, old)

	cfg, _ := settings.New(settings.Defaults())
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan net.Addr, 1)
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{
			Bind: "127.0.0.1", Port: 0, DataDir: base, Settings: cfg,
			TrustLoopback: true, Version: "test", Out: &out,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			Ready:  func(a net.Addr) { ready <- a },
		})
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("Run exited early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}
	resp, err := http.Get("http://" + addr.String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %d", resp.StatusCode)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale run directory was not removed")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not stop")
	}
	entries, _ := os.ReadDir(base)
	if len(entries) != 0 {
		t.Fatalf("data left behind: %v", entries)
	}
	if !strings.Contains(out.String(), "CONSIGNA") || !strings.Contains(out.String(), "this computer only") {
		t.Fatalf("banner:\n%s", out.String())
	}
}

func TestListenReportsBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if _, err := listen(context.Background(), "127.0.0.1", port); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("err = %v", err)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{24 * time.Hour: "24h", 72 * time.Hour: "3 days", 2 * time.Hour: "2h", 90 * time.Minute: "1h30m0s"}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
