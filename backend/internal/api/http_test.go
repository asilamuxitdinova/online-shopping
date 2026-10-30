package api

import (
	"net/http"
	"testing"

	"online-shopping/internal/config"
)

func TestHealth(t *testing.T) {
	e := newTestEnv(t)
	e.expect(http.StatusOK, "GET", "/api/health", "", nil)

	e.srv.db.Close()
	e.expect(http.StatusServiceUnavailable, "GET", "/api/health", "", nil)
}

func TestUnknownRoute(t *testing.T) {
	e := newTestEnv(t)
	res := e.expect(http.StatusNotFound, "PUT", "/api/products/1", "", nil)
	if res.errMsg(t) != "Route PUT /api/products/1 not found" {
		t.Fatalf("unexpected error %s", res.raw)
	}
}

func TestCORS(t *testing.T) {
	e := newTestEnv(t)

	res := e.do("GET", "/api/products", "", nil, "Origin", "https://shop.test")
	if got := res.header.Get("Access-Control-Allow-Origin"); got != "https://shop.test" {
		t.Errorf("allowed origin: got %q", got)
	}

	res = e.do("GET", "/api/products", "", nil, "Origin", "https://evil.test")
	if got := res.header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("other origin got CORS header %q", got)
	}

	res = e.do("OPTIONS", "/api/cart", "", nil,
		"Origin", "https://shop.test", "Access-Control-Request-Method", "POST")
	if res.status != http.StatusNoContent || res.header.Get("Access-Control-Allow-Headers") == "" {
		t.Errorf("preflight: got %d, headers %v", res.status, res.header)
	}

	wildcard := newTestEnv(t, func(c *config.Config) { c.CORSOrigins = []string{"*"} })
	res = wildcard.do("GET", "/api/products", "", nil, "Origin", "https://anything.test")
	if got := res.header.Get("Access-Control-Allow-Origin"); got != "https://anything.test" {
		t.Errorf("wildcard: got %q", got)
	}
}

func TestAuthRateLimit(t *testing.T) {
	e := newTestEnv(t, func(c *config.Config) { c.AuthRatePerMinute = 3 })
	badLogin := map[string]any{"email": adminEmail, "password": "wrong-password"}

	for range 3 {
		e.expect(http.StatusUnauthorized, "POST", "/api/auth/login", "", badLogin)
	}
	// X-Real-IP is ignored unless TRUST_PROXY is on, so it cannot be used to dodge the limit.
	res := e.do("POST", "/api/auth/login", "", badLogin, "X-Real-IP", "203.0.113.9")
	if res.status != http.StatusTooManyRequests || res.header.Get("Retry-After") == "" {
		t.Fatalf("got %d (Retry-After %q), want 429", res.status, res.header.Get("Retry-After"))
	}
	e.expect(http.StatusTooManyRequests, "POST", "/api/auth/register", "", map[string]any{})
	e.expect(http.StatusOK, "GET", "/api/products", "", nil) // other routes are not limited
}

func TestAuthRateLimitBehindProxy(t *testing.T) {
	e := newTestEnv(t, func(c *config.Config) {
		c.AuthRatePerMinute = 2
		c.TrustProxy = true
	})
	badLogin := map[string]any{"email": adminEmail, "password": "wrong-password"}
	from := func(ip string) int { return e.do("POST", "/api/auth/login", "", badLogin, "X-Real-IP", ip).status }

	from("198.51.100.1")
	from("198.51.100.1")
	if got := from("198.51.100.1"); got != http.StatusTooManyRequests {
		t.Fatalf("third attempt from same IP: got %d, want 429", got)
	}
	if got := from("198.51.100.2"); got != http.StatusUnauthorized {
		t.Fatalf("other IP: got %d, want 401", got)
	}
}
