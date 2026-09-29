package store

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type limits struct {
	limit atomic.Int64
	ttl   atomic.Int64
}

func (l *limits) TrayLimit() int64               { return l.limit.Load() }
func (l *limits) FileTTL() time.Duration         { return time.Duration(l.ttl.Load()) }
func (l *limits) set(n int64, ttl time.Duration) { l.limit.Store(n); l.ttl.Store(int64(ttl)) }

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

type fixture struct {
	s       *Store
	lim     *limits
	clk     *clock
	changes atomic.Int64
	dir     string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{lim: &limits{}, clk: &clock{now: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)}, dir: t.TempDir()}
	f.lim.set(1000, time.Hour)
	s, err := New(Options{
		Dir: f.dir, Limits: f.lim, Now: f.clk.Now,
		StaleUploadAfter: 30 * time.Minute,
		OnChange:         func() { f.changes.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	f.s = s
	return f
}

var alice = Owner{ID: "dev-alice", Name: "quiet-otter"}
var bob = Owner{ID: "dev-bob", Name: "blue-heron"}

func (f *fixture) upload(t *testing.T, owner Owner, name, body string) File {
	t.Helper()
	u, done, err := f.s.CreateUpload(owner, name, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if done != nil {
		return *done
	}
	_, file, err := f.s.WriteChunk(context.Background(), u.ID, owner.ID, 0, strings.NewReader(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	if file == nil {
		t.Fatal("upload did not complete")
	}
	return *file
}

func readAll(t *testing.T, s *Store, id string) string {
	t.Helper()
	r, _, err := s.Open(id)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUploadRoundTrip(t *testing.T) {
	f := newFixture(t)
	file := f.upload(t, alice, "../notes.txt", "hello tray")
	if file.Name != "notes.txt" || file.Size != 10 || file.UploaderName != "quiet-otter" {
		t.Fatalf("unexpected file %+v", file)
	}
	if want := f.clk.Now().Add(time.Hour); !file.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", file.ExpiresAt, want)
	}
	if got := readAll(t, f.s, file.ID); got != "hello tray" {
		t.Fatalf("content = %q", got)
	}
	if u := f.s.Usage(); u.Used != 10 || u.Reserved != 0 {
		t.Fatalf("usage = %+v", u)
	}
	if len(f.s.Uploads()) != 0 {
		t.Fatal("upload not cleaned up")
	}
	if entries, _ := os.ReadDir(filepath.Join(f.dir, "uploads")); len(entries) != 0 {
		t.Fatalf("partial files left: %d", len(entries))
	}
}

func TestZeroByteUploadCompletesImmediately(t *testing.T) {
	f := newFixture(t)
	u, file, err := f.s.CreateUpload(alice, "empty.txt", 0)
	if err != nil || file == nil {
		t.Fatalf("CreateUpload = %v, %v", file, err)
	}
	if u.Offset != 0 || u.Size != 0 {
		t.Fatalf("upload = %+v", u)
	}
	if got := readAll(t, f.s, file.ID); got != "" {
		t.Fatalf("content = %q", got)
	}
	if head, err := f.s.GetUpload(u.ID, alice.ID); err != nil || head.Offset != 0 || head.Size != 0 {
		t.Fatalf("GetUpload after zero-byte create = %+v, %v", head, err)
	}
}

func TestResumeAfterInterruptedChunk(t *testing.T) {
	f := newFixture(t)
	body := "0123456789abcdef"
	u, _, err := f.s.CreateUpload(alice, "a.bin", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	netErr := errors.New("connection reset")
	r := io.MultiReader(strings.NewReader(body[:6]), &failingReader{err: netErr})
	got, file, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 0, r, nil)
	if !errors.Is(err, netErr) || file != nil {
		t.Fatalf("WriteChunk = %v, %v", file, err)
	}
	if got.Offset != 6 {
		t.Fatalf("offset after failure = %d, want 6", got.Offset)
	}
	head, err := f.s.GetUpload(u.ID, alice.ID)
	if err != nil || head.Offset != 6 {
		t.Fatalf("GetUpload = %+v, %v", head, err)
	}
	if _, _, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 0, strings.NewReader(body), nil); !errors.Is(err, ErrOffsetMismatch) {
		t.Fatalf("stale offset: %v", err)
	}
	_, file, err = f.s.WriteChunk(context.Background(), u.ID, alice.ID, 6, strings.NewReader(body[6:]), nil)
	if err != nil || file == nil {
		t.Fatalf("resume = %v, %v", file, err)
	}
	if got := readAll(t, f.s, file.ID); got != body {
		t.Fatalf("content = %q", got)
	}
}

type failingReader struct{ err error }

func (r *failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestQuota(t *testing.T) {
	f := newFixture(t)
	f.lim.set(100, time.Hour)
	if _, _, err := f.s.CreateUpload(alice, "big", 101); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("oversized: %v", err)
	}
	u1, _, err := f.s.CreateUpload(alice, "one", 60)
	if err != nil {
		t.Fatal(err)
	}
	// The first upload's reservation counts even before any byte arrives.
	if _, _, err := f.s.CreateUpload(bob, "two", 41); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("parallel overshoot: %v", err)
	}
	if u := f.s.Usage(); u.Reserved != 60 || u.Limit != 100 {
		t.Fatalf("usage = %+v", u)
	}
	if err := f.s.TerminateUpload(u1.ID, alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.s.CreateUpload(bob, "two", 100); err != nil {
		t.Fatalf("after termination: %v", err)
	}
}

func TestTooLargeKeepsDeclaredBytes(t *testing.T) {
	f := newFixture(t)
	u, _, _ := f.s.CreateUpload(alice, "x", 4)
	_, file, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 0, strings.NewReader("abcdEXTRA"), nil)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if file == nil || readAll(t, f.s, file.ID) != "abcd" {
		t.Fatal("declared bytes were not kept")
	}
}

func TestUploadsArePrivateToOwner(t *testing.T) {
	f := newFixture(t)
	u, _, _ := f.s.CreateUpload(alice, "x", 4)
	if _, err := f.s.GetUpload(u.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUpload by other: %v", err)
	}
	if _, _, err := f.s.WriteChunk(context.Background(), u.ID, bob.ID, 0, strings.NewReader("abcd"), nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("WriteChunk by other: %v", err)
	}
	if err := f.s.TerminateUpload(u.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("TerminateUpload by other: %v", err)
	}
}

// blockingReader delivers its data, then blocks until interrupted.
type blockingReader struct {
	data    []byte
	release chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	<-r.release
	return 0, errors.New("interrupted")
}

func TestNewWriteTakesOverStuckWrite(t *testing.T) {
	f := newFixture(t)
	u, _, _ := f.s.CreateUpload(alice, "x.bin", 8)
	stuck := &blockingReader{data: []byte("1234"), release: make(chan struct{})}
	var once sync.Once
	interrupt := func() { once.Do(func() { close(stuck.release) }) }

	firstDone := make(chan Upload)
	go func() {
		got, _, _ := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 0, stuck, interrupt)
		firstDone <- got
	}()
	// Wait until the first write holds the upload.
	waitFor(t, func() bool {
		f.s.mu.Lock()
		defer f.s.mu.Unlock()
		return f.s.uploads[u.ID].writer != nil
	})
	// The second write names offset 4, which only becomes true once the
	// stuck write is interrupted and its 4 bytes are committed.
	_, file, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 4, strings.NewReader("5678"), nil)
	if err != nil || file == nil {
		t.Fatalf("takeover = %v, %v", file, err)
	}
	if got := <-firstDone; got.Offset != 4 {
		t.Fatalf("first write offset = %d", got.Offset)
	}
	if got := readAll(t, f.s, file.ID); got != "12345678" {
		t.Fatalf("content = %q", got)
	}
}

func TestTerminateDuringWrite(t *testing.T) {
	f := newFixture(t)
	u, _, _ := f.s.CreateUpload(alice, "x.bin", 8)
	stuck := &blockingReader{data: []byte("12"), release: make(chan struct{})}
	var once sync.Once
	done := make(chan error)
	go func() {
		_, _, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 0, stuck, func() { once.Do(func() { close(stuck.release) }) })
		done <- err
	}()
	waitFor(t, func() bool {
		f.s.mu.Lock()
		defer f.s.mu.Unlock()
		return f.s.uploads[u.ID].writer != nil
	})
	if err := f.s.TerminateUpload(u.ID, alice.ID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrNotFound) {
		t.Fatalf("writer err = %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.dir, "uploads")); len(entries) != 0 {
		t.Fatal("partial file left behind")
	}
	if u := f.s.Usage(); u.Reserved != 0 {
		t.Fatalf("reservation leaked: %+v", u)
	}
}

func TestDeleteWhileDownloading(t *testing.T) {
	f := newFixture(t)
	file := f.upload(t, alice, "a.txt", "contents")
	r, _, err := f.s.Open(file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n := f.s.Delete(file.ID, file.ID, "missing"); n != 1 {
		t.Fatalf("Delete removed %d", n)
	}
	if _, err := f.s.Get(file.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted file still listed")
	}
	b, err := io.ReadAll(r)
	if err != nil || string(b) != "contents" {
		t.Fatalf("read after delete = %q, %v", b, err)
	}
	blob := filepath.Join(f.dir, "files", r.entry.ID)
	if _, err := os.Stat(blob); err != nil {
		t.Fatal("blob removed while still open")
	}
	r.Close()
	r.Close() // idempotent
	if _, err := os.Stat(blob); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("blob not removed after last reader closed")
	}
	if u := f.s.Usage(); u.Used != 0 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestSweepExpiresFilesAndStaleUploads(t *testing.T) {
	f := newFixture(t)
	old := f.upload(t, alice, "old.txt", "old")
	f.clk.Advance(40 * time.Minute)
	fresh := f.upload(t, bob, "fresh.txt", "fresh")
	stale, _, _ := f.s.CreateUpload(alice, "stale.bin", 50)

	f.clk.Advance(20 * time.Minute) // old is now exactly 1h old
	f.s.Sweep()
	if _, err := f.s.Get(old.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired file kept")
	}
	if _, err := f.s.Get(fresh.ID); err != nil {
		t.Fatal("fresh file removed")
	}
	if _, err := f.s.GetUpload(stale.ID, alice.ID); err != nil {
		t.Fatal("upload dropped too early")
	}
	f.clk.Advance(11 * time.Minute)
	f.s.Sweep()
	if _, err := f.s.GetUpload(stale.ID, alice.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("stale upload kept")
	}
	if u := f.s.Usage(); u.Used != 5 || u.Reserved != 0 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestShorterTTLAppliesToExistingFiles(t *testing.T) {
	f := newFixture(t)
	file := f.upload(t, alice, "a", "a")
	f.clk.Advance(10 * time.Minute)
	f.lim.set(1000, 5*time.Minute)
	f.s.Sweep()
	if _, err := f.s.Get(file.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("file should expire under the new TTL")
	}
}

func TestClear(t *testing.T) {
	f := newFixture(t)
	f.upload(t, alice, "a", "aaa")
	f.s.CreateUpload(bob, "b", 10)
	f.s.Clear()
	if len(f.s.Files()) != 0 || len(f.s.Uploads()) != 0 {
		t.Fatal("tray not empty")
	}
	if u := f.s.Usage(); u.Used != 0 || u.Reserved != 0 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestTerminateUploadsOf(t *testing.T) {
	f := newFixture(t)
	a, _, _ := f.s.CreateUpload(alice, "a", 10)
	b, _, _ := f.s.CreateUpload(bob, "b", 10)
	f.s.TerminateUploadsOf(alice.ID)
	if _, err := f.s.GetUpload(a.ID, alice.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("alice's upload kept")
	}
	if _, err := f.s.GetUpload(b.ID, bob.ID); err != nil {
		t.Fatal("bob's upload removed")
	}
}

func TestFilesNewestFirst(t *testing.T) {
	f := newFixture(t)
	a := f.upload(t, alice, "a", "a")
	f.clk.Advance(time.Second)
	b := f.upload(t, alice, "b", "b")
	files := f.s.Files()
	if len(files) != 2 || files[0].ID != b.ID || files[1].ID != a.ID {
		t.Fatalf("order = %+v", files)
	}
}

func TestOnChangeFires(t *testing.T) {
	f := newFixture(t)
	before := f.changes.Load()
	file := f.upload(t, alice, "a", "a")
	f.s.Delete(file.ID)
	if f.changes.Load()-before < 3 { // create, complete, delete
		t.Fatalf("changes = %d", f.changes.Load()-before)
	}
}

func TestClosedStoreRejectsWork(t *testing.T) {
	f := newFixture(t)
	f.s.Close()
	if _, _, err := f.s.CreateUpload(alice, "a", 1); !errors.Is(err, ErrClosed) {
		t.Fatalf("err = %v", err)
	}
}

func TestConcurrentUploadsAndDeletes(t *testing.T) {
	f := newFixture(t)
	f.lim.set(1<<40, time.Hour)
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			owner := Owner{ID: string(rune('a' + i%26)), Name: "n"}
			body := bytes.Repeat([]byte{byte(i)}, 1000+i)
			u, _, err := f.s.CreateUpload(owner, "f", int64(len(body)))
			if err != nil {
				t.Error(err)
				return
			}
			_, file, err := f.s.WriteChunk(context.Background(), u.ID, owner.ID, 0, bytes.NewReader(body), nil)
			if err != nil || file == nil {
				t.Error("upload failed", err)
				return
			}
			if i%2 == 0 {
				f.s.Delete(file.ID)
			}
			f.s.Sweep()
			_ = f.s.Files()
		}()
	}
	wg.Wait()
	if n := len(f.s.Files()); n != 16 {
		t.Fatalf("files = %d, want 16", n)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCompletedUploadStaysVisibleForRetries(t *testing.T) {
	f := newFixture(t)
	u, _, _ := f.s.CreateUpload(alice, "a.txt", 3)
	if _, file, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 0, strings.NewReader("abc"), nil); err != nil || file == nil {
		t.Fatalf("WriteChunk = %v, %v", file, err)
	}
	// The client lost the response and asks again.
	head, err := f.s.GetUpload(u.ID, alice.ID)
	if err != nil || head.Offset != 3 || head.Size != 3 {
		t.Fatalf("GetUpload = %+v, %v", head, err)
	}
	if _, err := f.s.GetUpload(u.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("completed upload visible to another device")
	}
	again, file, err := f.s.WriteChunk(context.Background(), u.ID, alice.ID, 3, strings.NewReader(""), nil)
	if err != nil || file != nil || again.Offset != 3 {
		t.Fatalf("repeat PATCH = %+v, %v, %v", again, file, err)
	}
	if n := len(f.s.Files()); n != 1 {
		t.Fatalf("files = %d, want 1", n)
	}
	f.clk.Advance(31 * time.Minute)
	f.s.Sweep()
	if _, err := f.s.GetUpload(u.ID, alice.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("completed record not pruned")
	}
}
