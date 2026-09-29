package main

import (
	"bytes"
	"context"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/EymanEP/consigna/internal/sizes"
)

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "90m": 90 * time.Minute, "0.5d": 12 * time.Hour}
	for in, want := range cases {
		got, err := parseDuration(in)
		if err != nil || got != want {
			t.Errorf("parseDuration(%q) = %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "d", "-1d", "soon"} {
		if _, err := parseDuration(bad); err == nil {
			t.Errorf("parseDuration(%q) succeeded", bad)
		}
	}
}

func TestApplyEnv(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	port := fs.Int("port", 7431, "")
	bind := fs.String("bind", "0.0.0.0", "")
	noQR := fs.Bool("no-qr", false, "")
	if err := fs.Parse([]string{"-bind", "127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"CONSIGNA_PORT": "8080", "CONSIGNA_BIND": "10.0.0.1", "CONSIGNA_NO_QR": "true"}
	if err := applyEnv(fs, func(k string) (string, bool) { v, ok := env[k]; return v, ok }); err != nil {
		t.Fatal(err)
	}
	if *port != 8080 || *bind != "127.0.0.1" || !*noQR {
		t.Fatalf("port=%d bind=%s noQR=%v", *port, *bind, *noQR)
	}
	env["CONSIGNA_PORT"] = "nope"
	fs2 := flag.NewFlagSet("t", flag.ContinueOnError)
	fs2.Int("port", 1, "")
	if err := applyEnv(fs2, func(k string) (string, bool) { v, ok := env[k]; return v, ok }); err == nil {
		t.Fatal("invalid env accepted")
	}
}

func TestLoadSettingsFlagsOverride(t *testing.T) {
	s, path, err := loadSettings(options{noSettings: true, trayLimit: "2GB", ttl: "2h"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "" || s.TrayLimit() != 2*sizes.GB || s.FileTTL() != 2*time.Hour {
		t.Fatalf("got %q %d %v", path, s.TrayLimit(), s.FileTTL())
	}
	if _, _, err := loadSettings(options{noSettings: true, trayLimit: "1KB"}); err == nil {
		t.Fatal("tiny tray limit accepted")
	}
}

func TestVersionAndBadArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"-version"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "consigna ") {
		t.Fatalf("version output %q", out.String())
	}
	if err := run(context.Background(), []string{"extra"}, &out, &errOut); err == nil {
		t.Fatal("extra argument accepted")
	}
	if err := run(context.Background(), []string{"-log-format", "xml", "-no-settings-file"}, &out, &errOut); err == nil {
		t.Fatal("bad log format accepted")
	}
}
