// Command consigna shares files between devices on the same network.
//
// Run it on one computer, open the link it prints (or scan its QR code) on
// your other devices, and drop files into the shared tray.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/EymanEP/consigna/internal/app"
	"github.com/EymanEP/consigna/internal/settings"
	"github.com/EymanEP/consigna/internal/sizes"
	"github.com/EymanEP/consigna/web"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	os.Exit(mainCode())
}

func mainCode() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "consigna:", err)
		return 1
	}
	return 0
}

type options struct {
	port          int
	bind          string
	trayLimit     string
	ttl           string
	dataDir       string
	settingsPath  string
	noSettings    bool
	noQR          bool
	trustLoopback bool
	allowHosts    string
	logLevel      string
	logFormat     string
	showVersion   bool
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("consigna", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o options
	fs.IntVar(&o.port, "port", 7431, "TCP port to listen on")
	fs.StringVar(&o.bind, "bind", "0.0.0.0", "IP address to listen on (127.0.0.1 = this computer only)")
	fs.StringVar(&o.trayLimit, "tray-limit", "", "maximum size of the shared tray for this run, e.g. 10GB (default: saved setting, else 10GB)")
	fs.StringVar(&o.ttl, "ttl", "", "how long files stay in the tray for this run, e.g. 24h or 7d (default: saved setting, else 24h)")
	fs.StringVar(&o.dataDir, "data-dir", "", "where tray contents are kept while running (default: per-user cache dir)")
	fs.StringVar(&o.settingsPath, "settings", "", "settings file changed from the host view (default: per-user config dir)")
	fs.BoolVar(&o.noSettings, "no-settings-file", false, "do not read or save a settings file")
	fs.BoolVar(&o.noQR, "no-qr", false, "do not print a QR code in the terminal")
	fs.BoolVar(&o.trustLoopback, "trust-loopback", true, "treat browsers on this computer as the host; disable behind a reverse proxy")
	fs.StringVar(&o.allowHosts, "allow-host", "", "comma-separated host names allowed besides IP addresses and localhost")
	fs.StringVar(&o.logLevel, "log-level", "info", "log level: debug, info, warn or error")
	fs.StringVar(&o.logFormat, "log-format", "text", "log format: text or json")
	fs.BoolVar(&o.showVersion, "version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: consigna [flags]\n\nShare files between devices on the same network.\n\nFlags (also settable as CONSIGNA_<FLAG>, e.g. CONSIGNA_PORT=8080):\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if err := applyEnv(fs, os.LookupEnv); err != nil {
		return err
	}
	if o.showVersion {
		fmt.Fprintln(stdout, "consigna", version)
		return nil
	}
	if o.port < 0 || o.port > 65535 {
		return fmt.Errorf("--port must be between 0 and 65535")
	}

	logger, err := newLogger(stderr, o.logLevel, o.logFormat)
	if err != nil {
		return err
	}

	store, path, err := loadSettings(o)
	if err != nil {
		return err
	}

	dataDir := o.dataDir
	if dataDir == "" {
		if dataDir, err = app.DefaultDataDir(); err != nil {
			return fmt.Errorf("find a data directory (use --data-dir): %w", err)
		}
	}

	return app.Run(ctx, app.Config{
		Bind:          o.bind,
		Port:          o.port,
		DataDir:       dataDir,
		Settings:      store,
		SettingsPath:  path,
		TrustLoopback: o.trustLoopback,
		AllowHosts:    splitList(o.allowHosts),
		ShowQR:        !o.noQR,
		Version:       version,
		Web:           web.FS(),
		Logger:        logger,
		Out:           stdout,
	})
}

// applyEnv fills flags that were not given on the command line from
// CONSIGNA_* environment variables.
func applyEnv(fs *flag.FlagSet, lookup func(string) (string, bool)) error {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	var err error
	fs.VisitAll(func(f *flag.Flag) {
		if set[f.Name] || err != nil {
			return
		}
		key := "CONSIGNA_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, ok := lookup(key); ok {
			if e := fs.Set(f.Name, v); e != nil {
				err = fmt.Errorf("%s: %w", key, e)
			}
		}
	})
	return err
}

func loadSettings(o options) (*settings.Store, string, error) {
	var (
		s    *settings.Store
		path string
		err  error
	)
	if o.noSettings {
		s, err = settings.New(settings.Defaults())
	} else {
		path = o.settingsPath
		if path == "" {
			if path, err = settings.DefaultPath(); err != nil {
				return nil, "", fmt.Errorf("find a settings location (use --settings or --no-settings-file): %w", err)
			}
		}
		s, err = settings.Load(path)
	}
	if err != nil {
		return nil, "", err
	}
	v := s.Get()
	if o.trayLimit != "" {
		if v.TrayLimit, err = sizes.Parse(o.trayLimit); err != nil {
			return nil, "", fmt.Errorf("--tray-limit: %w", err)
		}
	}
	if o.ttl != "" {
		if v.FileTTL, err = parseDuration(o.ttl); err != nil {
			return nil, "", fmt.Errorf("--ttl: %w", err)
		}
	}
	if err := s.Override(v); err != nil {
		return nil, "", err
	}
	return s, path, nil
}

// parseDuration extends time.ParseDuration with a "d" (day) unit.
func parseDuration(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(strings.TrimSpace(s), "d"); ok {
		n, err := strconv.ParseFloat(days, 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}

func newLogger(w io.Writer, level, format string) (*slog.Logger, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("--log-level: %w", err)
	}
	opts := &slog.HandlerOptions{Level: l}
	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("--log-format must be text or json")
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
