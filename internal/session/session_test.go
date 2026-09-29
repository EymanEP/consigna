package session

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newManager(t *testing.T) (*Manager, *clock, *atomic.Int64) {
	t.Helper()
	clk := &clock{now: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)}
	var changes atomic.Int64
	m := New(Options{Now: clk.Now, OnChange: func() { changes.Add(1) }, MaxDevices: 4, FailuresPerIP: 3, GlobalFailures: 5})
	return m, clk, &changes
}

func TestCodeShape(t *testing.T) {
	m, _, _ := newManager(t)
	code := m.Code()
	if len(code) != CodeLength {
		t.Fatalf("code %q", code)
	}
	for _, r := range code {
		if !strings.ContainsRune(codeAlphabet, r) {
			t.Fatalf("code %q has %q", code, r)
		}
	}
}

func TestJoinAndAuthenticate(t *testing.T) {
	m, clk, changes := newManager(t)
	token, d, err := m.Join(strings.ToLower(m.Code()), "192.168.1.20", "Mozilla/5.0 (iPhone)")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name == "" || d.ID == "" || d.IsHost {
		t.Fatalf("device %+v", d)
	}
	if changes.Load() != 1 {
		t.Fatalf("changes = %d", changes.Load())
	}
	clk.Advance(time.Minute)
	got, ok := m.Authenticate(token, "192.168.1.21", "")
	if !ok || got.ID != d.ID {
		t.Fatalf("Authenticate = %+v, %v", got, ok)
	}
	if got.IP != "192.168.1.21" || !got.LastSeen.Equal(clk.Now()) || got.UserAgent != "Mozilla/5.0 (iPhone)" {
		t.Fatalf("activity not recorded: %+v", got)
	}
	if _, ok := m.Authenticate("nope", "", ""); ok {
		t.Fatal("bad token accepted")
	}
	if _, ok := m.Authenticate("", "", ""); ok {
		t.Fatal("empty token accepted")
	}
}

func TestWrongCodeIsRateLimitedPerIP(t *testing.T) {
	m, clk, _ := newManager(t)
	for range 3 {
		if _, _, err := m.Join("WRONG1", "10.0.0.1", ""); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("err = %v", err)
		}
	}
	// Even the right code is refused while the IP is locked out.
	if _, _, err := m.Join(m.Code(), "10.0.0.1", ""); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
	// Other IPs are unaffected.
	if _, _, err := m.Join(m.Code(), "10.0.0.2", ""); err != nil {
		t.Fatalf("other IP: %v", err)
	}
	clk.Advance(10 * time.Minute)
	if _, _, err := m.Join(m.Code(), "10.0.0.1", ""); err != nil {
		t.Fatalf("after window: %v", err)
	}
}

func TestGlobalRateLimit(t *testing.T) {
	m, _, _ := newManager(t)
	for i := range 5 {
		m.Join("WRONG1", "10.0.1."+string(rune('0'+i)), "")
	}
	if _, _, err := m.Join(m.Code(), "10.0.2.1", ""); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
}

func TestMaxDevices(t *testing.T) {
	m, _, _ := newManager(t)
	for range 4 {
		if _, _, err := m.JoinTrusted("127.0.0.1", "", false); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.Join(m.Code(), "10.0.0.9", ""); !errors.Is(err, ErrFull) {
		t.Fatalf("err = %v", err)
	}
}

func TestRename(t *testing.T) {
	m, _, _ := newManager(t)
	_, a, _ := m.JoinTrusted("127.0.0.1", "", true)
	_, b, _ := m.JoinTrusted("127.0.0.1", "", false)
	got, err := m.Rename(a.ID, "  Work   laptop \u200b ")
	if err != nil || got.Name != "Work laptop" {
		t.Fatalf("Rename = %+v, %v", got, err)
	}
	if _, err := m.Rename(b.ID, "work LAPTOP"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := m.Rename(a.ID, "work laptop"); err != nil {
		t.Fatalf("renaming to own name in other case: %v", err)
	}
	for _, bad := range []string{"", "   ", "\u200b", strings.Repeat("x", MaxNameRunes+1)} {
		if _, err := m.Rename(a.ID, bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Rename(%q) = %v", bad, err)
		}
	}
	if _, err := m.Rename("missing", "ok"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestPresence(t *testing.T) {
	m, _, changes := newManager(t)
	_, d, _ := m.JoinTrusted("127.0.0.1", "", false)
	base := changes.Load()
	r1 := m.Connect(d.ID)
	r2 := m.Connect(d.ID)
	if got, _ := m.Get(d.ID); !got.Online {
		t.Fatal("not online")
	}
	r1()
	r1() // idempotent
	if got, _ := m.Get(d.ID); !got.Online {
		t.Fatal("offline while a tab is still open")
	}
	r2()
	if got, _ := m.Get(d.ID); got.Online {
		t.Fatal("still online")
	}
	if n := changes.Load() - base; n != 2 {
		t.Fatalf("presence changes = %d, want 2", n)
	}
	m.Connect("missing")() // no panic
}

func TestRemoveAndEnd(t *testing.T) {
	m, _, _ := newManager(t)
	ta, a, _ := m.JoinTrusted("127.0.0.1", "", false)
	tb, _, _ := m.JoinTrusted("127.0.0.1", "", false)
	release := m.Connect(a.ID)
	if err := m.Remove(a.ID); err != nil {
		t.Fatal(err)
	}
	release() // releasing after removal must not panic or resurrect
	if _, ok := m.Authenticate(ta, "", ""); ok {
		t.Fatal("removed device still authenticates")
	}
	if err := m.Remove(a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	old := m.Code()
	if m.RotateCode() == old {
		t.Fatal("code did not rotate")
	}
	if _, ok := m.Authenticate(tb, "", ""); !ok {
		t.Fatal("rotation signed devices out")
	}
	m.End()
	if _, ok := m.Authenticate(tb, "", ""); ok {
		t.Fatal("End did not sign devices out")
	}
	if len(m.Devices()) != 0 {
		t.Fatal("devices left after End")
	}
}

func TestDevicesOrderedByJoin(t *testing.T) {
	m, clk, _ := newManager(t)
	_, a, _ := m.JoinTrusted("", "", false)
	clk.Advance(time.Second)
	_, b, _ := m.JoinTrusted("", "", false)
	ds := m.Devices()
	if len(ds) != 2 || ds[0].ID != a.ID || ds[1].ID != b.ID {
		t.Fatalf("order %+v", ds)
	}
	if ds[0].Name == ds[1].Name {
		t.Fatal("names not unique")
	}
}
