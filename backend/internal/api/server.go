// Package api implements the shop's HTTP endpoints.
package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"online-shopping/internal/config"
)

type Server struct {
	db          *sql.DB
	cfg         *config.Config
	authLimiter *ipRateLimiter
}

func New(db *sql.DB, cfg *config.Config) *Server {
	return &Server{db: db, cfg: cfg, authLimiter: newIPRateLimiter(cfg.AuthRatePerMinute)}
}

// Handler returns the router with all routes and middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h handlerFunc) { mux.HandleFunc(pattern, s.handle(h)) }

	route("GET /api/health", s.health)

	route("POST /api/auth/register", s.rateLimited(s.register))
	route("POST /api/auth/login", s.rateLimited(s.login))
	route("POST /api/auth/refresh", s.rateLimited(s.refresh))
	route("POST /api/auth/logout", s.logout)
	route("POST /api/auth/logout-all", s.requireAuth(s.logoutAll))
	route("GET /api/auth/me", s.requireAuth(s.me))

	route("GET /api/products", s.listProducts)
	route("GET /api/products/categories", s.listCategories)
	route("GET /api/products/{id}", s.getProduct)
	route("POST /api/products", s.requireAdmin(s.createProduct))
	route("PATCH /api/products/{id}", s.requireAdmin(s.updateProduct))
	route("DELETE /api/products/{id}", s.requireAdmin(s.deleteProduct))

	route("GET /api/cart", s.requireAuth(s.getCart))
	route("POST /api/cart/items", s.requireAuth(s.addCartItem))
	route("PATCH /api/cart/items/{productId}", s.requireAuth(s.updateCartItem))
	route("DELETE /api/cart/items/{productId}", s.requireAuth(s.removeCartItem))
	route("DELETE /api/cart", s.requireAuth(s.clearCart))

	route("POST /api/orders", s.requireAuth(s.createOrder))
	route("GET /api/orders", s.requireAuth(s.listOrders))
	route("GET /api/orders/{id}", s.requireAuth(s.getOrder))
	route("POST /api/orders/{id}/cancel", s.requireAuth(s.cancelOrder))
	route("PATCH /api/orders/{id}/status", s.requireAdmin(s.updateOrderStatus))

	route("/", func(w http.ResponseWriter, r *http.Request) error {
		return httpErr(http.StatusNotFound, "Route %s %s not found", r.Method, r.URL.Path)
	})

	return logRequests(s.cors(mux))
}

// GET /api/health  — 503 when the database is unreachable
func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		log.Printf("health: database unreachable: %v", err)
		return respond(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
	}
	return respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
