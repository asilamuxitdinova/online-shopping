package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"online-shopping/internal/config"
	"online-shopping/internal/seed"
	"online-shopping/internal/testdb"
)

const (
	adminEmail    = "admin@shop.test"
	adminPassword = "admin-password-123"
	userPassword  = "password1"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // request logs and migration messages
	os.Exit(m.Run())
}

// testEnv is a running API backed by its own seeded database.
type testEnv struct {
	t   *testing.T
	srv *Server
	url string
}

func newTestEnv(t *testing.T, opts ...func(*config.Config)) *testEnv {
	t.Helper()
	conn := testdb.New(t)
	cfg := &config.Config{
		JWTSecret:         []byte(strings.Repeat("s", 32)),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   time.Hour,
		CORSOrigins:       []string{"https://shop.test"},
		AuthRatePerMinute: 10_000,
		AdminEmail:        adminEmail,
		AdminPassword:     adminPassword,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if err := seed.Run(context.Background(), conn, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}

	srv := New(conn, cfg)
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return &testEnv{t: t, srv: srv, url: hs.URL}
}

type response struct {
	status int
	header http.Header
	raw    []byte
}

// decode unmarshals the response body into v.
func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.raw, v); err != nil {
		t.Fatalf("decode %s: %v", r.raw, err)
	}
}

// errMsg returns the "error" field of an error response.
func (r response) errMsg(t *testing.T) string {
	t.Helper()
	var body struct{ Error string }
	r.decode(t, &body)
	return body.Error
}

// send performs a request without failing the test, so it can be used from goroutines.
func (e *testEnv) send(method, path, token string, body any, headers ...string) (response, error) {
	var rd io.Reader
	if body != nil {
		if s, ok := body.(string); ok {
			rd = strings.NewReader(s)
		} else {
			b, err := json.Marshal(body)
			if err != nil {
				return response{}, err
			}
			rd = bytes.NewReader(b)
		}
	}
	req, err := http.NewRequest(method, e.url+path, rd)
	if err != nil {
		return response{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return response{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	return response{status: res.StatusCode, header: res.Header, raw: raw}, err
}

func (e *testEnv) do(method, path, token string, body any, headers ...string) response {
	e.t.Helper()
	res, err := e.send(method, path, token, body, headers...)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	return res
}

// expect performs a request and fails the test unless it returns status.
func (e *testEnv) expect(status int, method, path, token string, body any) response {
	e.t.Helper()
	res := e.do(method, path, token, body)
	if res.status != status {
		e.t.Fatalf("%s %s: got %d, want %d; body: %s", method, path, res.status, status, res.raw)
	}
	return res
}

func (e *testEnv) register(email string) authResponse {
	e.t.Helper()
	var auth authResponse
	e.expect(http.StatusCreated, "POST", "/api/auth/register", "",
		map[string]any{"name": "Test User", "email": email, "password": userPassword}).decode(e.t, &auth)
	return auth
}

func (e *testEnv) login(email, password string) authResponse {
	e.t.Helper()
	var auth authResponse
	e.expect(http.StatusOK, "POST", "/api/auth/login", "",
		map[string]any{"email": email, "password": password}).decode(e.t, &auth)
	return auth
}

func (e *testEnv) adminToken() string {
	e.t.Helper()
	return e.login(adminEmail, adminPassword).AccessToken
}

func (e *testEnv) product(id int64) Product {
	e.t.Helper()
	var body struct{ Product Product }
	e.expect(http.StatusOK, "GET", fmt.Sprintf("/api/products/%d", id), "", nil).decode(e.t, &body)
	return body.Product
}

func (e *testEnv) createProduct(token string, fields map[string]any) Product {
	e.t.Helper()
	var body struct{ Product Product }
	e.expect(http.StatusCreated, "POST", "/api/products", token, fields).decode(e.t, &body)
	return body.Product
}

func (e *testEnv) addToCart(token string, productID, quantity int64) Cart {
	e.t.Helper()
	var cart Cart
	e.expect(http.StatusCreated, "POST", "/api/cart/items", token,
		map[string]any{"productId": productID, "quantity": quantity}).decode(e.t, &cart)
	return cart
}

func (e *testEnv) checkout(token string) Order {
	e.t.Helper()
	var body struct{ Order Order }
	e.expect(http.StatusCreated, "POST", "/api/orders", token,
		map[string]any{"shippingAddress": "Tashkent, Amir Temur 1"}).decode(e.t, &body)
	return body.Order
}

// parallel runs fn n times concurrently and counts the resulting status codes.
func parallel(t *testing.T, n int, fn func() (response, error)) map[int]int {
	t.Helper()
	type result struct {
		status int
		err    error
	}
	results := make(chan result, n)
	for range n {
		go func() {
			res, err := fn()
			results <- result{res.status, err}
		}()
	}
	counts := map[int]int{}
	for range n {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		counts[r.status]++
	}
	return counts
}
