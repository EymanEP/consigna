package server

import (
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

var errInterrupted = errors.New("transfer interrupted")

// uploadBody wraps a request body for long uploads.
//
// Before every read it pushes the connection's read deadline forward, so an
// upload may take as long as it needs but a connection that stops delivering
// data is cut after timeout. Phones that lose Wi-Fi mid-upload otherwise
// leave half-open connections around for many minutes.
//
// Interrupt stops the upload from another goroutine, e.g. when the same
// client reconnects and resumes. Finish must be called before the handler
// returns: after that the connection may serve another request and must not
// be touched.
type uploadBody struct {
	r       io.Reader
	rc      *http.ResponseController
	timeout time.Duration

	mu          sync.Mutex
	interrupted bool
	finished    bool
}

func newUploadBody(r io.Reader, rc *http.ResponseController, timeout time.Duration) *uploadBody {
	return &uploadBody{r: r, rc: rc, timeout: timeout}
}

func (b *uploadBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	if b.interrupted {
		b.mu.Unlock()
		return 0, errInterrupted
	}
	_ = b.rc.SetReadDeadline(time.Now().Add(b.timeout))
	b.mu.Unlock()
	return b.r.Read(p)
}

// Interrupt makes the current and all future reads fail.
func (b *uploadBody) Interrupt() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished || b.interrupted {
		return
	}
	b.interrupted = true
	_ = b.rc.SetReadDeadline(time.Now())
}

// Finish detaches the body from the connection and reports whether it was
// interrupted.
func (b *uploadBody) Finish() (interrupted bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.finished && !b.interrupted {
		_ = b.rc.SetReadDeadline(time.Time{})
	}
	b.finished = true
	return b.interrupted
}

// idleWriter is the download-side counterpart: each write pushes the write
// deadline forward.
type idleWriter struct {
	http.ResponseWriter
	rc      *http.ResponseController
	timeout time.Duration
}

func (i idleWriter) Write(p []byte) (int, error) {
	_ = i.rc.SetWriteDeadline(time.Now().Add(i.timeout))
	return i.ResponseWriter.Write(p)
}

func (i idleWriter) Unwrap() http.ResponseWriter { return i.ResponseWriter }
