package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	runPrefix = "run-"
	aliveFile = ".alive"
	// staleAfter is how old a run's heartbeat must be before another run
	// may treat it as crashed and delete its files.
	staleAfter = 10 * time.Minute
	// heartbeatEvery refreshes the heartbeat well within staleAfter.
	heartbeatEvery = time.Minute
)

// DefaultDataDir is a per-user cache location. It is preferred over the
// system temp directory, which is often a small RAM disk shared by all users.
func DefaultDataDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "consigna"), nil
}

// runDir is this process's private directory for tray contents.
type runDir struct {
	path string
	log  *slog.Logger
}

// newRunDir creates a fresh run directory under base and removes the
// leftovers of runs that crashed without cleaning up.
func newRunDir(base string, log *slog.Logger) (*runDir, error) {
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	removeStaleRuns(base, log)
	path, err := os.MkdirTemp(base, runPrefix)
	if err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}
	d := &runDir{path: path, log: log}
	if err := d.touch(); err != nil {
		_ = os.RemoveAll(path)
		return nil, err
	}
	return d, nil
}

// touch refreshes the heartbeat so concurrent runs know this one is alive.
func (d *runDir) touch() error {
	p := filepath.Join(d.path, aliveFile)
	now := time.Now()
	if err := os.Chtimes(p, now, now); err == nil {
		return nil
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}
	return nil
}

// remove deletes the run directory and everything in it. It retries briefly
// because on Windows a file handle can take a moment to be released.
func (d *runDir) remove() error {
	var err error
	for attempt := range 5 {
		if err = os.RemoveAll(d.path); err == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	d.log.Warn("could not remove data", "dir", d.path, "err", err)
	return err
}

func removeStaleRuns(base string, log *slog.Logger) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), runPrefix) {
			continue
		}
		dir := filepath.Join(base, e.Name())
		info, err := os.Stat(filepath.Join(dir, aliveFile))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err == nil && time.Since(info.ModTime()) < staleAfter {
			continue // another Consigna is running
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Warn("could not remove leftover data", "dir", dir, "err", err)
		} else {
			log.Info("removed leftover data from an earlier run", "dir", dir)
		}
	}
}
