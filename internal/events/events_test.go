package events

import "testing"

func TestHubCoalescesAndNeverBlocks(t *testing.T) {
	var h Hub
	s := h.Subscribe()
	defer s.Close()
	select {
	case <-s.C:
	default:
		t.Fatal("no initial wake-up")
	}
	for range 100 {
		h.Notify()
	}
	<-s.C
	select {
	case <-s.C:
		t.Fatal("wake-ups were not coalesced")
	default:
	}
}

func TestClose(t *testing.T) {
	var h Hub
	s := h.Subscribe()
	if h.Len() != 1 {
		t.Fatal("not registered")
	}
	s.Close()
	s.Close()
	if h.Len() != 0 {
		t.Fatal("not unregistered")
	}
	h.Notify()
}
