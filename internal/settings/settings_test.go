package settings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EymanEP/consigna/internal/sizes"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get(); got != Defaults() {
		t.Fatalf("got %+v, want defaults", got)
	}
}

func TestUpdatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Values{TrayLimit: 3 * sizes.GB, FileTTL: 2 * time.Hour}
	if err := s.Update(want); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Get(); got != want {
		t.Fatalf("reloaded %+v, want %+v", got, want)
	}
}

func TestUpdateRejectsOutOfBounds(t *testing.T) {
	s, _ := New(Defaults())
	bad := []Values{
		{TrayLimit: 1, FileTTL: time.Hour},
		{TrayLimit: sizes.GB, FileTTL: time.Second},
		{TrayLimit: sizes.GB, FileTTL: 365 * 24 * time.Hour},
	}
	for _, v := range bad {
		if err := s.Update(v); !errors.Is(err, ErrInvalid) {
			t.Errorf("Update(%+v) = %v, want ErrInvalid", v, err)
		}
	}
	if s.Get() != Defaults() {
		t.Fatal("rejected update changed values")
	}
}

func TestLoadRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for corrupt file")
	}
	if err := os.WriteFile(path, []byte(`{"trayLimitBytes":5,"fileTtlSeconds":60}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestOverrideDoesNotPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Load(path)
	if err := s.Override(Values{TrayLimit: sizes.GB, FileTTL: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("override wrote a file: %v", err)
	}
}
