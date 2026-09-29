package session

import "time"

// limiter counts failed join attempts per IP and overall within a fixed
// window. It is not safe for concurrent use; the Manager's lock guards it.
type limiter struct {
	perIP  int
	global int
	window time.Duration
	byIP   map[string]*window
	all    window
}

type window struct {
	start time.Time
	count int
}

func newLimiter(perIP, global int, win time.Duration) *limiter {
	return &limiter{perIP: perIP, global: global, window: win, byIP: map[string]*window{}}
}

func (l *limiter) allowed(ip string, now time.Time) bool {
	l.prune(now)
	if l.all.count >= l.global {
		return false
	}
	if w, ok := l.byIP[ip]; ok && w.count >= l.perIP {
		return false
	}
	return true
}

func (l *limiter) fail(ip string, now time.Time) {
	if l.all.count == 0 {
		l.all.start = now
	}
	l.all.count++
	w, ok := l.byIP[ip]
	if !ok {
		w = &window{start: now}
		l.byIP[ip] = w
	}
	w.count++
}

func (l *limiter) prune(now time.Time) {
	if l.all.count > 0 && now.Sub(l.all.start) >= l.window {
		l.all = window{}
	}
	for ip, w := range l.byIP {
		if now.Sub(w.start) >= l.window {
			delete(l.byIP, ip)
		}
	}
}
