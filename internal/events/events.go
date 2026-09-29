// Package events fans out "something changed" signals to live connections.
//
// Subscribers are not sent the change itself. Each one receives a coalescing
// wake-up and then renders the current state for its own viewer. This keeps
// publishers non-blocking, never drops a client for being slow, and lets a
// burst of changes collapse into a single update.
package events

import "sync"

// Hub is safe for concurrent use. The zero value is ready to use.
type Hub struct {
	mu   sync.Mutex
	subs map[*Subscription]struct{}
}

// Subscription receives wake-ups on C until it is closed.
type Subscription struct {
	// C holds at most one pending wake-up.
	C   chan struct{}
	hub *Hub
}

// Subscribe registers a new subscriber. It starts with a pending wake-up so
// the first state is sent immediately.
func (h *Hub) Subscribe() *Subscription {
	s := &Subscription{C: make(chan struct{}, 1), hub: h}
	s.C <- struct{}{}
	h.mu.Lock()
	if h.subs == nil {
		h.subs = map[*Subscription]struct{}{}
	}
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Close unregisters the subscription. It is safe to call more than once.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	delete(s.hub.subs, s)
	s.hub.mu.Unlock()
}

// Notify wakes every subscriber. It never blocks.
func (h *Hub) Notify() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.C <- struct{}{}:
		default: // a wake-up is already pending
		}
	}
}

// Len reports the number of subscribers.
func (h *Hub) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
