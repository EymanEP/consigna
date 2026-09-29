// Package store keeps the shared tray: finished files on disk, uploads in
// progress, the tray quota and file expiry.
//
// File contents live under a directory owned by the store and are always
// addressed by random IDs, never by client-supplied names. Metadata lives in
// memory only: the tray is temporary by design and is discarded on restart.
package store

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Errors returned by the store. Callers map them to protocol responses.
var (
	ErrNotFound       = errors.New("store: not found")
	ErrQuotaExceeded  = errors.New("store: not enough space left in the tray")
	ErrOffsetMismatch = errors.New("store: upload offset mismatch")
	ErrLocked         = errors.New("store: upload is busy")
	ErrTooLarge       = errors.New("store: more data than the declared upload length")
	ErrInvalidSize    = errors.New("store: invalid upload size")
	ErrClosed         = errors.New("store: closed")
)

// takeoverTimeout bounds how long a new write waits for an interrupted,
// older write on the same upload to let go.
const takeoverTimeout = 10 * time.Second

// Limits supplies the current, host-adjustable limits.
type Limits interface {
	TrayLimit() int64
	FileTTL() time.Duration
}

// Owner identifies the device that creates an upload.
type Owner struct {
	ID   string
	Name string
}

// File is a finished file in the tray.
type File struct {
	ID           string
	Name         string
	Size         int64
	UploaderID   string
	UploaderName string
	AddedAt      time.Time
	ExpiresAt    time.Time
}

// Upload is an upload in progress.
type Upload struct {
	ID        string
	Name      string
	Size      int64
	Offset    int64
	OwnerID   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Usage describes how full the tray is. Reserved counts the declared size of
// uploads still in progress, so parallel uploads cannot overshoot the limit.
type Usage struct {
	Used     int64
	Reserved int64
	Limit    int64
}

// Options configure a Store.
type Options struct {
	// Dir is where contents are written. It is created if missing.
	Dir string
	// Limits provides the tray limit and file expiry. Required.
	Limits Limits
	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
	// StaleUploadAfter is how long an idle, unfinished upload is kept.
	StaleUploadAfter time.Duration
	// OnChange is called, outside any lock, whenever the visible state of
	// the tray changes (files added or removed, usage changed).
	OnChange func()
	// Logger receives operational messages. Defaults to slog.Default().
	Logger *slog.Logger
}

type fileEntry struct {
	File
	path    string
	refs    int
	removed bool
}

type uploadEntry struct {
	Upload
	ownerName string
	path      string
	writer    *writer
	removed   bool
}

type writer struct {
	done      chan struct{}
	interrupt func()
}

// Store is safe for concurrent use.
type Store struct {
	opts      Options
	blobDir   string
	uploadDir string

	mu        sync.Mutex
	files     map[string]*fileEntry
	uploads   map[string]*uploadEntry
	completed map[string]completedUpload
	used      int64
	reserved  int64
	closed    bool
}

// completedUpload remembers a finished upload for a while, so a client that
// never saw the final response learns on retry that it is done instead of
// starting over and creating a duplicate.
type completedUpload struct {
	Upload
	at time.Time
}

// New creates a store rooted at opts.Dir.
func New(opts Options) (*Store, error) {
	if opts.Limits == nil {
		return nil, errors.New("store: Limits is required")
	}
	if opts.Dir == "" {
		return nil, errors.New("store: Dir is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.StaleUploadAfter <= 0 {
		opts.StaleUploadAfter = 6 * time.Hour
	}
	if opts.OnChange == nil {
		opts.OnChange = func() {}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	s := &Store{
		opts:      opts,
		blobDir:   filepath.Join(opts.Dir, "files"),
		uploadDir: filepath.Join(opts.Dir, "uploads"),
		files:     map[string]*fileEntry{},
		uploads:   map[string]*uploadEntry{},
		completed: map[string]completedUpload{},
	}
	for _, d := range []string{s.blobDir, s.uploadDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("store: %w", err)
		}
	}
	return s, nil
}

// Files returns every file in the tray, newest first.
func (s *Store) Files() []File {
	s.mu.Lock()
	defer s.mu.Unlock()
	ttl := s.opts.Limits.FileTTL()
	out := make([]File, 0, len(s.files))
	for _, e := range s.files {
		f := e.File
		f.ExpiresAt = f.AddedAt.Add(ttl)
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b File) int {
		if c := b.AddedAt.Compare(a.AddedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// Usage reports how full the tray is.
func (s *Store) Usage() Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Usage{Used: s.used, Reserved: s.reserved, Limit: s.opts.Limits.TrayLimit()}
}

// Uploads returns the uploads in progress, oldest first.
func (s *Store) Uploads() []Upload {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Upload, 0, len(s.uploads))
	for _, u := range s.uploads {
		out = append(out, u.Upload)
	}
	slices.SortFunc(out, func(a, b Upload) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out
}

// CreateUpload reserves space for a new upload of exactly size bytes. A
// zero-byte upload is finished immediately and the resulting file returned.
func (s *Store) CreateUpload(owner Owner, name string, size int64) (Upload, *File, error) {
	if size < 0 {
		return Upload{}, nil, ErrInvalidSize
	}
	name = SanitizeName(name)
	now := s.opts.Now()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Upload{}, nil, ErrClosed
	}
	if s.used+s.reserved+size > s.opts.Limits.TrayLimit() {
		s.mu.Unlock()
		return Upload{}, nil, ErrQuotaExceeded
	}
	id := newID()
	u := &uploadEntry{
		Upload: Upload{
			ID: id, Name: name, Size: size, OwnerID: owner.ID,
			CreatedAt: now, UpdatedAt: now,
		},
		ownerName: owner.Name,
		path:      filepath.Join(s.uploadDir, id),
	}
	if err := createEmpty(u.path); err != nil {
		s.mu.Unlock()
		return Upload{}, nil, err
	}
	if size == 0 {
		f, err := s.finalizeLocked(u)
		if err == nil {
			s.completed[id] = completedUpload{Upload: u.Upload, at: now}
		}
		s.mu.Unlock()
		if err != nil {
			return Upload{}, nil, err
		}
		s.opts.OnChange()
		return u.Upload, &f, nil
	}
	s.uploads[id] = u
	s.reserved += size
	s.mu.Unlock()
	s.opts.OnChange()
	return u.Upload, nil, nil
}

// GetUpload returns an upload owned by ownerID. A recently finished upload is
// still reported, with its offset equal to its size.
func (s *Store) GetUpload(id, ownerID string) (Upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.uploads[id]; ok && u.OwnerID == ownerID {
		return u.Upload, nil
	}
	if c, ok := s.completed[id]; ok && c.OwnerID == ownerID {
		return c.Upload, nil
	}
	return Upload{}, ErrNotFound
}

// WriteChunk appends body to the upload, which must currently be at offset.
//
// Only one write per upload runs at a time. When a new write arrives while an
// older one is still attached (typically a phone that reconnected before the
// server noticed the old connection died) the older write is interrupted via
// its interrupt callback and the new one takes over.
//
// Bytes received before an error are kept, so the returned Upload always
// carries the offset to resume from. When the upload completes, the finished
// File is returned as well.
func (s *Store) WriteChunk(ctx context.Context, id, ownerID string, offset int64, body io.Reader, interrupt func()) (Upload, *File, error) {
	s.mu.Lock()
	if c, ok := s.completed[id]; ok && c.OwnerID == ownerID && !s.closed {
		s.mu.Unlock()
		if offset != c.Size {
			return c.Upload, nil, ErrOffsetMismatch
		}
		return c.Upload, nil, nil
	}
	u, err := s.lookupUploadLocked(id, ownerID)
	if err != nil {
		s.mu.Unlock()
		return Upload{}, nil, err
	}
	if prev := u.writer; prev != nil {
		s.mu.Unlock()
		if prev.interrupt != nil {
			prev.interrupt()
		}
		timer := time.NewTimer(takeoverTimeout)
		select {
		case <-prev.done:
			timer.Stop()
		case <-timer.C:
			return Upload{}, nil, ErrLocked
		case <-ctx.Done():
			timer.Stop()
			return Upload{}, nil, ctx.Err()
		}
		s.mu.Lock()
		if u, err = s.lookupUploadLocked(id, ownerID); err != nil {
			s.mu.Unlock()
			return Upload{}, nil, err
		}
		if u.writer != nil {
			s.mu.Unlock()
			return Upload{}, nil, ErrLocked
		}
	}
	if u.Offset != offset {
		cur := u.Upload
		s.mu.Unlock()
		return cur, nil, ErrOffsetMismatch
	}
	w := &writer{done: make(chan struct{}), interrupt: interrupt}
	u.writer = w
	remaining := u.Size - u.Offset
	path := u.path
	s.mu.Unlock()

	n, writeErr := appendAt(path, offset, body, remaining)

	s.mu.Lock()
	u.writer = nil
	close(w.done)
	if u.removed {
		s.mu.Unlock()
		removeQuietly(path)
		return Upload{}, nil, ErrNotFound
	}
	u.Offset += n
	u.UpdatedAt = s.opts.Now()
	cur := u.Upload
	// Once every declared byte is on disk the upload is complete; the only
	// error still possible then is ErrTooLarge, which is reported after
	// finishing so the valid data is not thrown away.
	if u.Offset < u.Size {
		s.mu.Unlock()
		return cur, nil, writeErr
	}
	delete(s.uploads, u.ID)
	s.reserved -= u.Size
	f, err := s.finalizeLocked(u)
	if err == nil {
		s.completed[u.ID] = completedUpload{Upload: cur, at: cur.UpdatedAt}
	}
	s.mu.Unlock()
	s.opts.OnChange()
	if err != nil {
		return cur, nil, err
	}
	return cur, &f, writeErr
}

// TerminateUpload cancels an upload owned by ownerID and frees its space.
func (s *Store) TerminateUpload(id, ownerID string) error {
	s.mu.Lock()
	if c, ok := s.completed[id]; ok && c.OwnerID == ownerID {
		delete(s.completed, id)
		s.mu.Unlock()
		return nil
	}
	u, err := s.lookupUploadLocked(id, ownerID)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.dropUploadLocked(u)
	s.mu.Unlock()
	s.opts.OnChange()
	return nil
}

// TerminateUploadsOf cancels every upload owned by ownerID.
func (s *Store) TerminateUploadsOf(ownerID string) {
	s.mu.Lock()
	changed := false
	for _, u := range s.uploads {
		if u.OwnerID == ownerID {
			s.dropUploadLocked(u)
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.opts.OnChange()
	}
}

// Reader streams a file's contents. Close must be called.
type Reader struct {
	*os.File
	store *Store
	entry *fileEntry
	once  sync.Once
}

// Close releases the file. Deleting a file while it is being downloaded is
// allowed: the contents are removed from disk once the last reader closes,
// which also keeps Windows (which cannot delete open files) happy.
func (r *Reader) Close() error {
	var err error
	r.once.Do(func() {
		err = r.File.Close()
		r.store.mu.Lock()
		r.entry.refs--
		gone := r.entry.removed && r.entry.refs == 0
		r.store.mu.Unlock()
		if gone {
			removeQuietly(r.entry.path)
		}
	})
	return err
}

// Open returns a reader for a file in the tray.
func (s *Store) Open(id string) (*Reader, File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.files[id]
	if !ok || s.closed {
		return nil, File{}, ErrNotFound
	}
	f, err := os.Open(e.path)
	if err != nil {
		return nil, File{}, fmt.Errorf("store: open %s: %w", id, err)
	}
	e.refs++
	file := e.File
	file.ExpiresAt = file.AddedAt.Add(s.opts.Limits.FileTTL())
	return &Reader{File: f, store: s, entry: e}, file, nil
}

// Get returns a file's metadata.
func (s *Store) Get(id string) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.files[id]
	if !ok {
		return File{}, ErrNotFound
	}
	f := e.File
	f.ExpiresAt = f.AddedAt.Add(s.opts.Limits.FileTTL())
	return f, nil
}

// Delete removes files from the tray. Unknown IDs are ignored; the number of
// files actually removed is returned.
func (s *Store) Delete(ids ...string) int {
	s.mu.Lock()
	var gone []string
	n := 0
	for _, id := range ids {
		if e, ok := s.files[id]; ok {
			n++
			if p := s.removeFileLocked(e); p != "" {
				gone = append(gone, p)
			}
		}
	}
	s.mu.Unlock()
	for _, p := range gone {
		removeQuietly(p)
	}
	if n > 0 {
		s.opts.OnChange()
	}
	return n
}

// Clear removes every file and upload, e.g. when the host ends the session.
func (s *Store) Clear() {
	s.mu.Lock()
	var gone []string
	for _, e := range s.files {
		if p := s.removeFileLocked(e); p != "" {
			gone = append(gone, p)
		}
	}
	for _, u := range s.uploads {
		s.dropUploadLocked(u)
	}
	clear(s.completed)
	s.mu.Unlock()
	for _, p := range gone {
		removeQuietly(p)
	}
	s.opts.OnChange()
}

// Sweep removes expired files and stale uploads as of now.
func (s *Store) Sweep() {
	now := s.opts.Now()
	s.mu.Lock()
	ttl := s.opts.Limits.FileTTL()
	var gone []string
	changed := false
	for _, e := range s.files {
		if !now.Before(e.AddedAt.Add(ttl)) {
			if p := s.removeFileLocked(e); p != "" {
				gone = append(gone, p)
			}
			changed = true
			s.opts.Logger.Info("file expired", "file", e.ID, "size", e.Size)
		}
	}
	for _, u := range s.uploads {
		if u.writer == nil && !now.Before(u.UpdatedAt.Add(s.opts.StaleUploadAfter)) {
			s.dropUploadLocked(u)
			changed = true
			s.opts.Logger.Info("stale upload dropped", "upload", u.ID, "offset", u.Offset, "size", u.Size)
		}
	}
	for id, c := range s.completed {
		if !now.Before(c.at.Add(s.opts.StaleUploadAfter)) {
			delete(s.completed, id)
		}
	}
	s.mu.Unlock()
	for _, p := range gone {
		removeQuietly(p)
	}
	if changed {
		s.opts.OnChange()
	}
}

// Run sweeps periodically until ctx is done.
func (s *Store) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Sweep()
		}
	}
}

// Close interrupts running uploads and makes further calls fail. It does not
// delete the directory; the owner of the directory does that.
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, u := range s.uploads {
		if u.writer != nil && u.writer.interrupt != nil {
			u.writer.interrupt()
		}
	}
}

func (s *Store) lookupUploadLocked(id, ownerID string) (*uploadEntry, error) {
	if s.closed {
		return nil, ErrClosed
	}
	u, ok := s.uploads[id]
	if !ok || u.OwnerID != ownerID {
		return nil, ErrNotFound
	}
	return u, nil
}

// dropUploadLocked forgets an upload and frees its reservation. If a write is
// running, it is interrupted and the writer deletes the partial file when it
// returns; otherwise the partial file is deleted here.
func (s *Store) dropUploadLocked(u *uploadEntry) {
	delete(s.uploads, u.ID)
	s.reserved -= u.Size
	u.removed = true
	if u.writer != nil {
		if u.writer.interrupt != nil {
			u.writer.interrupt()
		}
		return
	}
	removeQuietly(u.path)
}

// removeFileLocked forgets a file and returns its path if the caller should
// delete it now (no reader holds it open).
func (s *Store) removeFileLocked(e *fileEntry) string {
	delete(s.files, e.ID)
	s.used -= e.Size
	e.removed = true
	if e.refs == 0 {
		return e.path
	}
	return ""
}

// finalizeLocked moves a completed upload's contents into the tray.
func (s *Store) finalizeLocked(u *uploadEntry) (File, error) {
	id := newID()
	path := filepath.Join(s.blobDir, id)
	if err := os.Rename(u.path, path); err != nil {
		removeQuietly(u.path)
		return File{}, fmt.Errorf("store: finalize upload: %w", err)
	}
	e := &fileEntry{
		File: File{
			ID: id, Name: u.Name, Size: u.Size,
			UploaderID: u.OwnerID, UploaderName: u.ownerName,
			AddedAt: s.opts.Now(),
		},
		path: path,
	}
	s.files[id] = e
	s.used += e.Size
	f := e.File
	f.ExpiresAt = f.AddedAt.Add(s.opts.Limits.FileTTL())
	return f, nil
}

func createEmpty(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("store: create upload: %w", err)
	}
	return f.Close()
}

// appendAt writes at most limit bytes from r into path starting at offset. It
// reports ErrTooLarge if r holds more than limit bytes.
func appendAt(path string, offset int64, r io.Reader, limit int64) (int64, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return 0, fmt.Errorf("store: open upload: %w", err)
	}
	n, copyErr := io.Copy(io.NewOffsetWriter(f, offset), io.LimitReader(r, limit))
	closeErr := f.Close()
	if copyErr != nil {
		return n, copyErr
	}
	if closeErr != nil {
		// The data may not have reached the disk; commit nothing so the
		// client re-sends this part.
		return 0, fmt.Errorf("store: write upload: %w", closeErr)
	}
	if n == limit {
		var probe [1]byte
		if m, _ := io.ReadFull(r, probe[:]); m > 0 {
			return n, ErrTooLarge
		}
	}
	return n, nil
}

func removeQuietly(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("could not remove file", "path", path, "err", err)
	}
}

var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// newID returns a random, URL-safe identifier with 128 bits of entropy.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return idEncoding.EncodeToString(b[:])
}
