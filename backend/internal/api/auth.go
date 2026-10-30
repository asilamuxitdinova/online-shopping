package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"online-shopping/internal/db"
)

var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type ctxKey struct{}

// currentUser returns the user set by requireAuth.
func currentUser(r *http.Request) User {
	return r.Context().Value(ctxKey{}).(User)
}

// POST /api/auth/register  { name, email, password }
func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}

	name := strings.TrimSpace(body.Name)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	switch {
	case name == "":
		return httpErr(http.StatusBadRequest, "Name is required")
	case tooLong(name, maxNameLen):
		return httpErr(http.StatusBadRequest, "Name must be at most %d characters", maxNameLen)
	case !emailRE.MatchString(email) || tooLong(email, maxEmailLen):
		return httpErr(http.StatusBadRequest, "A valid email is required")
	case len(body.Password) < 8:
		return httpErr(http.StatusBadRequest, "Password must be at least 8 characters")
	case len(body.Password) > 72:
		return httpErr(http.StatusBadRequest, "Password must be at most 72 bytes")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	ctx := r.Context()
	var resp authResponse
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var u User
		err := tx.QueryRowContext(ctx,
			`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
			 RETURNING id, name, email, role, created_at`,
			name, email, string(hash),
		).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
		if db.IsUniqueViolation(err) {
			return httpErr(http.StatusConflict, "Email is already registered")
		}
		if err != nil {
			return err
		}
		resp, err = s.issueTokens(ctx, tx, u)
		return err
	})
	if err != nil {
		return err
	}
	return respond(w, http.StatusCreated, resp)
}

// POST /api/auth/login  { email, password }
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || body.Password == "" {
		return httpErr(http.StatusBadRequest, "Email and password are required")
	}

	var u User
	var hash string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT id, name, email, role, created_at, password_hash FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		return httpErr(http.StatusUnauthorized, "Invalid email or password")
	}

	resp, err := s.issueTokens(r.Context(), s.db, u)
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, resp)
}

// GET /api/auth/me
func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	return respond(w, http.StatusOK, map[string]User{"user": currentUser(r)})
}

// requireAuth needs a valid "Authorization: Bearer <token>" header and stores the user in the context.
func (s *Server) requireAuth(next handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		scheme, token, _ := strings.Cut(r.Header.Get("Authorization"), " ")
		if scheme != "Bearer" || token == "" {
			return httpErr(http.StatusUnauthorized, "Authentication required")
		}

		var c claims
		_, err := jwt.ParseWithClaims(token, &c,
			func(*jwt.Token) (any, error) { return s.cfg.JWTSecret, nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			return httpErr(http.StatusUnauthorized, "Invalid or expired token")
		}
		id, err := strconv.ParseInt(c.Subject, 10, 64)
		if err != nil {
			return httpErr(http.StatusUnauthorized, "Invalid or expired token")
		}

		var u User
		err = s.db.QueryRowContext(r.Context(),
			`SELECT id, name, email, role, created_at FROM users WHERE id = $1`, id,
		).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return httpErr(http.StatusUnauthorized, "User no longer exists")
		}
		if err != nil {
			return err
		}

		return next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	}
}

func (s *Server) requireAdmin(next handlerFunc) handlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) error {
		if currentUser(r).Role != "admin" {
			return httpErr(http.StatusForbidden, "Admin access required")
		}
		return next(w, r)
	})
}
