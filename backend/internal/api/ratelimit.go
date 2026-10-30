package api

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipRateLimiter keeps one token bucket per client IP. Buckets idle for
// longer than ipIdleTimeout are dropped so the map does not grow forever.
type ipRateLimiter struct {
	mu        sync.Mutex
	limit     rate.Limit
	burst     int
	clients   map[string]*rateClient
	lastSweep time.Time
}

type rateClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

const ipIdleTimeout = 10 * time.Minute

// newIPRateLimiter allows perMinute requests per IP, all of which may come at once.
func newIPRateLimiter(perMinute int) *ipRateLimiter {
	return &ipRateLimiter{
		limit:     rate.Limit(float64(perMinute) / 60),
		burst:     perMinute,
		clients:   map[string]*rateClient{},
		lastSweep: time.Now(),
	}
}

// allow reports whether ip may make a request now and, if not, how long it must wait.
func (l *ipRateLimiter) allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.Sub(l.lastSweep) > time.Minute {
		for key, c := range l.clients {
			if now.Sub(c.lastSeen) > ipIdleTimeout {
				delete(l.clients, key)
			}
		}
		l.lastSweep = now
	}

	c, ok := l.clients[ip]
	if !ok {
		c = &rateClient{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.clients[ip] = c
	}
	c.lastSeen = now

	res := c.limiter.ReserveN(now, 1)
	if delay := res.DelayFrom(now); delay > 0 {
		res.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// clientIP returns the caller's IP. X-Real-IP is only trusted when the API runs
// behind a reverse proxy that sets it; otherwise any client could fake it.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimited rejects requests over the per-IP auth limit with 429.
func (s *Server) rateLimited(next handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if ok, wait := s.authLimiter.allow(s.clientIP(r)); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			return httpErr(http.StatusTooManyRequests, "Too many attempts, try again later")
		}
		return next(w, r)
	}
}
