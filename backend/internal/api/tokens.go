package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"online-shopping/internal/db"
)

// authResponse is returned by register, login and refresh.
type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // access token lifetime in seconds
	User         User   `json:"user"`
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Server) issueAccessToken(u User) (string, error) {
	now := time.Now()
	c := claims{
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(u.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.cfg.JWTSecret)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// issueTokens creates an access token and a new refresh token for u.
func (s *Server) issueTokens(ctx context.Context, q db.Querier, u User) (authResponse, error) {
	access, err := s.issueAccessToken(u)
	if err != nil {
		return authResponse{}, err
	}

	raw := make([]byte, 32)
	rand.Read(raw)
	refresh := base64.RawURLEncoding.EncodeToString(raw)

	if _, err := q.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1 AND expires_at <= now()`, u.ID); err != nil {
		return authResponse{}, err
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		u.ID, hashToken(refresh), time.Now().Add(s.cfg.RefreshTokenTTL)); err != nil {
		return authResponse{}, err
	}

	return authResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
		User:         u,
	}, nil
}

func revokeAllRefreshTokens(ctx context.Context, q db.Querier, userID int64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// POST /api/auth/refresh  { refreshToken }
// Rotates the refresh token: the one sent is revoked and a new pair is returned.
// Presenting an already revoked token revokes every session of that user, since
// it means the token was copied and used by someone else.
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) error {
	var body refreshRequest
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}
	if body.RefreshToken == "" {
		return httpErr(http.StatusBadRequest, "Refresh token is required")
	}

	ctx := r.Context()
	var resp authResponse
	var reused bool
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		var tokenID, userID int64
		var expired, revoked bool
		err := tx.QueryRowContext(ctx,
			`SELECT id, user_id, expires_at <= now(), revoked_at IS NOT NULL
			 FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, hashToken(body.RefreshToken),
		).Scan(&tokenID, &userID, &expired, &revoked)
		if errors.Is(err, sql.ErrNoRows) {
			return httpErr(http.StatusUnauthorized, "Invalid refresh token")
		}
		if err != nil {
			return err
		}
		if revoked {
			// Commit the revocation, then report the error after the transaction.
			reused = true
			return revokeAllRefreshTokens(ctx, tx, userID)
		}
		if expired {
			return httpErr(http.StatusUnauthorized, "Refresh token has expired")
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, tokenID); err != nil {
			return err
		}

		var u User
		if err := tx.QueryRowContext(ctx,
			`SELECT id, name, email, role, created_at FROM users WHERE id = $1`, userID,
		).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return err
		}
		resp, err = s.issueTokens(ctx, tx, u)
		return err
	})
	if err != nil {
		return err
	}
	if reused {
		return httpErr(http.StatusUnauthorized, "Refresh token was already used; all sessions have been signed out")
	}
	return respond(w, http.StatusOK, resp)
}

// Logging out deletes refresh tokens rather than revoking them, so a logged-out
// token is simply invalid instead of triggering reuse detection.

// POST /api/auth/logout  { refreshToken }  — ends this session
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	var body refreshRequest
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}
	if body.RefreshToken == "" {
		return httpErr(http.StatusBadRequest, "Refresh token is required")
	}
	if _, err := s.db.ExecContext(r.Context(),
		`DELETE FROM refresh_tokens WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashToken(body.RefreshToken)); err != nil {
		return err
	}
	return respond(w, http.StatusNoContent, nil)
}

// POST /api/auth/logout-all  — ends every session of the current user
func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.db.ExecContext(r.Context(),
		`DELETE FROM refresh_tokens WHERE user_id = $1`, currentUser(r).ID); err != nil {
		return err
	}
	return respond(w, http.StatusNoContent, nil)
}
