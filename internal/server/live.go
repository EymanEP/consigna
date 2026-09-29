package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// heartbeatEvery keeps idle connections (and proxies) from timing out and
// lets a dead client be noticed.
const heartbeatEvery = 20 * time.Second

// handleEvents streams the tray's state to a device with Server-Sent Events.
// The full state is sent on connect and after every change, so a client that
// reconnects after sleeping never needs to catch up on missed events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	me := deviceFrom(r)
	rc := http.NewResponseController(w)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	sub := s.hub.Subscribe()
	defer sub.Close()
	release := s.sessions.Connect(me.ID)
	defer release()

	send := func(chunk string) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprint(w, chunk); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send("retry: 2000\n\n") {
		return
	}

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopping:
			send("event: shutdown\ndata: {}\n\n")
			return
		case <-heartbeat.C:
			if !send(": ping\n\n") {
				return
			}
		case <-sub.C:
			d, ok := s.sessions.Get(me.ID)
			if !ok {
				send("event: ended\ndata: {}\n\n")
				return
			}
			data, err := json.Marshal(s.buildState(r, d))
			if err != nil {
				s.log.Error("encode state", "err", err)
				return
			}
			if !send("event: state\ndata: " + string(data) + "\n\n") {
				return
			}
		}
	}
}
