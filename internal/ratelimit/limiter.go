package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

// DenyReason describes why a connection was rejected.
// An empty string means the connection is allowed.
type DenyReason string

const (
	DenyReasonNone  DenyReason = ""
	DenyReasonTotal DenyReason = "server is at capacity, please try again later"
	DenyReasonIP    DenyReason = "too many connections from your IP, please try again later"
)

// Limiter enforces two independent limits before a WebSocket upgrade is allowed:
//
//  1. A global cap on total simultaneous WebSocket connections.
//  2. A per-IP cap on simultaneous WebSocket connections from a single address.
//
// Both limits are tracked with counters that are incremented on Allow and
// decremented when the connection closes (via Done). No timers, no token
// buckets — this is purely a concurrency limiter, not a request-rate limiter.
type Limiter struct {
	maxTotal int64
	maxPerIP int64

	total atomic.Int64
	mu    sync.Mutex
	perIP map[string]int64 // IP → active connection count
}

// New creates a Limiter with the given caps.
func New(maxTotal, maxPerIP int64) *Limiter {
	return &Limiter{
		maxTotal: maxTotal,
		maxPerIP: maxPerIP,
		perIP:    make(map[string]int64),
	}
}

// Allow checks whether a new connection from r.RemoteAddr should be permitted.
//
// On success it increments both counters and returns DenyReasonNone. The
// caller MUST call Done(r) when the connection eventually closes.
//
// On failure it returns a human-readable DenyReason without modifying any
// counter. The caller decides how to communicate the rejection to the client —
// this method has no knowledge of HTTP or WebSocket.
func (l *Limiter) Allow(r *http.Request) DenyReason {
	ip := extractIP(r)

	// Global counter — cheap atomic, no lock.
	if l.total.Add(1) > l.maxTotal {
		l.total.Add(-1)
		return DenyReasonTotal
	}

	// Per-IP counter — needs the lock.
	l.mu.Lock()
	if l.perIP[ip]+1 > l.maxPerIP {
		l.mu.Unlock()
		l.total.Add(-1) // roll back global too
		return DenyReasonIP
	}
	l.perIP[ip]++
	l.mu.Unlock()

	return DenyReasonNone
}

// Done decrements both counters for the IP in r.RemoteAddr.
// Must be called exactly once per successful Allow, typically via defer.
func (l *Limiter) Done(r *http.Request) {
	ip := extractIP(r)

	l.total.Add(-1)

	l.mu.Lock()
	l.perIP[ip]--
	if l.perIP[ip] == 0 {
		delete(l.perIP, ip) // prevent unbounded map growth from rotating IPs
	}
	l.mu.Unlock()
}

// Stats returns the current global connection count and a per-IP snapshot.
// Intended for health checks or metrics endpoints.
func (l *Limiter) Stats() (total int64, perIP map[string]int64) {
	l.mu.Lock()
	snap := make(map[string]int64, len(l.perIP))
	for k, v := range l.perIP {
		snap[k] = v
	}
	l.mu.Unlock()
	return l.total.Load(), snap
}

// extractIP pulls the IP address from r.RemoteAddr, stripping the port.
// Behind a reverse proxy, check X-Forwarded-For / X-Real-IP in upstream
// middleware before this runs — do not trust those headers blindly here.
func extractIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
