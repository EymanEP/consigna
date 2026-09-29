// Package settings holds the host-adjustable limits of a Consigna server and
// persists them between runs.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/EymanEP/consigna/internal/sizes"
)

// Defaults applied when nothing else is configured.
const (
	DefaultTrayLimit = 10 * sizes.GB
	DefaultFileTTL   = 24 * time.Hour
)

// Bounds enforced on every change so a typo cannot make the server unusable.
const (
	MinTrayLimit = 10 * sizes.MB
	MaxTrayLimit = 100 * sizes.TB
	MinFileTTL   = time.Minute
	MaxFileTTL   = 30 * 24 * time.Hour
)

// ErrInvalid wraps every validation failure.
var ErrInvalid = errors.New("invalid settings")

// Values are the adjustable limits.
type Values struct {
	// TrayLimit is the maximum number of bytes the shared tray may hold,
	// counting files and uploads in progress.
	TrayLimit int64
	// FileTTL is how long a file stays in the tray after it arrives.
	FileTTL time.Duration
}

// Defaults returns the built-in values.
func Defaults() Values {
	return Values{TrayLimit: DefaultTrayLimit, FileTTL: DefaultFileTTL}
}

// Validate reports whether v is within the allowed bounds.
func (v Values) Validate() error {
	if v.TrayLimit < MinTrayLimit || v.TrayLimit > MaxTrayLimit {
		return fmt.Errorf("%w: tray limit must be between %s and %s",
			ErrInvalid, sizes.Format(MinTrayLimit), sizes.Format(MaxTrayLimit))
	}
	if v.FileTTL < MinFileTTL || v.FileTTL > MaxFileTTL {
		return fmt.Errorf("%w: file expiry must be between %s and %s",
			ErrInvalid, MinFileTTL, MaxFileTTL)
	}
	return nil
}

// fileFormat is the on-disk JSON representation.
type fileFormat struct {
	TrayLimitBytes int64 `json:"trayLimitBytes"`
	FileTTLSeconds int64 `json:"fileTtlSeconds"`
}

// Store is a concurrency-safe holder for Values, optionally backed by a JSON
// file. The zero value is not usable; call New or Load.
type Store struct {
	mu   sync.RWMutex
	path string
	v    Values
}

// New returns an in-memory store that never touches disk.
func New(v Values) (*Store, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return &Store{v: v}, nil
}

// Load reads settings from path. A missing file yields the defaults; a file
// that is unreadable or out of bounds is reported as an error so a broken
// configuration is never silently ignored.
func Load(path string) (*Store, error) {
	s := &Store{path: path, v: Defaults()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse settings %s: %w", path, err)
	}
	v := Values{
		TrayLimit: f.TrayLimitBytes,
		FileTTL:   time.Duration(f.FileTTLSeconds) * time.Second,
	}
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("settings %s: %w", path, err)
	}
	s.v = v
	return s, nil
}

// Get returns the current values.
func (s *Store) Get() Values {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v
}

// TrayLimit returns the current tray limit in bytes.
func (s *Store) TrayLimit() int64 { return s.Get().TrayLimit }

// FileTTL returns the current file expiry.
func (s *Store) FileTTL() time.Duration { return s.Get().FileTTL }

// Override replaces the values for this run only, without persisting them.
// It is used for command-line flags.
func (s *Store) Override(v Values) error {
	if err := v.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v = v
	return nil
}

// Update validates, applies and persists v. When the store has no backing
// file the change only lives in memory.
func (s *Store) Update(v Values) error {
	if err := v.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path != "" {
		if err := writeFile(s.path, v); err != nil {
			return err
		}
	}
	s.v = v
	return nil
}

// writeFile atomically replaces path with the JSON encoding of v.
func writeFile(path string, v Values) error {
	data, err := json.MarshalIndent(fileFormat{
		TrayLimitBytes: v.TrayLimit,
		FileTTLSeconds: int64(v.FileTTL / time.Second),
	}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.json")
	if err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}

// DefaultPath returns the per-user location of the settings file.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "consigna", "settings.json"), nil
}
