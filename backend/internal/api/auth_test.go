package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"online-shopping/internal/config"
)

func TestRegisterValidation(t *testing.T) {
	e := newTestEnv(t)
	tests := []struct {
		name string
		body any
		want string
	}{
		{"empty body", nil, "Name is required"},
		{"blank name", map[string]any{"name": " ", "email": "a@b.co", "password": userPassword}, "Name is required"},
		{"long name", map[string]any{"name": strings.Repeat("x", maxNameLen+1), "email": "a@b.co", "password": userPassword}, "Name must be at most"},
		{"bad email", map[string]any{"name": "A", "email": "not-an-email", "password": userPassword}, "A valid email is required"},
		{"short password", map[string]any{"name": "A", "email": "a@b.co", "password": "short"}, "at least 8"},
		{"long password", map[string]any{"name": "A", "email": "a@b.co", "password": strings.Repeat("p", 73)}, "at most 72"},
		{"invalid JSON", "{bad", "Invalid JSON body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := e.do("POST", "/api/auth/register", "", tt.body)
			if res.status != http.StatusBadRequest || !strings.Contains(res.errMsg(t), tt.want) {
				t.Fatalf("got %d %s, want 400 containing %q", res.status, res.raw, tt.want)
			}
		})
	}
}

func TestRegisterLoginAndMe(t *testing.T) {
	e := newTestEnv(t)

	auth := e.register("  New.User@Shop.TEST ")
	if auth.User.Email != "new.user@shop.test" || auth.User.Role != "customer" {
		t.Fatalf("unexpected user %+v", auth.User)
	}
	if auth.TokenType != "Bearer" || auth.ExpiresIn != 900 || auth.AccessToken == "" || auth.RefreshToken == "" {
		t.Fatalf("unexpected token response %+v", auth)
	}

	res := e.expect(http.StatusConflict, "POST", "/api/auth/register", "",
		map[string]any{"name": "Again", "email": "NEW.USER@shop.test", "password": userPassword})
	if res.errMsg(t) != "Email is already registered" {
		t.Fatalf("unexpected error %s", res.raw)
	}

	e.expect(http.StatusUnauthorized, "POST", "/api/auth/login", "",
		map[string]any{"email": "new.user@shop.test", "password": "wrong-password"})
	e.expect(http.StatusUnauthorized, "POST", "/api/auth/login", "",
		map[string]any{"email": "nobody@shop.test", "password": userPassword})
	e.expect(http.StatusBadRequest, "POST", "/api/auth/login", "", map[string]any{})

	login := e.login("new.user@shop.test", userPassword)
	var me struct{ User User }
	e.expect(http.StatusOK, "GET", "/api/auth/me", login.AccessToken, nil).decode(t, &me)
	if me.User.ID != auth.User.ID {
		t.Fatalf("me returned user %d, want %d", me.User.ID, auth.User.ID)
	}
}

func TestAccessTokenRejected(t *testing.T) {
	e := newTestEnv(t)
	user := e.register("user@shop.test").User

	expired := *e.srv.cfg
	expired.AccessTokenTTL = -time.Minute
	expiredToken, _ := (&Server{cfg: &expired}).issueAccessToken(user)

	otherKey := *e.srv.cfg
	otherKey.JWTSecret = []byte(strings.Repeat("x", 32))
	forgedToken, _ := (&Server{cfg: &otherKey}).issueAccessToken(user)

	for name, header := range map[string]string{
		"missing":     "",
		"not bearer":  "Basic abc",
		"garbage":     "Bearer abc",
		"expired":     "Bearer " + expiredToken,
		"wrong key":   "Bearer " + forgedToken,
		"empty token": "Bearer ",
	} {
		t.Run(name, func(t *testing.T) {
			res := e.do("GET", "/api/auth/me", "", nil, "Authorization", header)
			if res.status != http.StatusUnauthorized {
				t.Fatalf("got %d, want 401", res.status)
			}
		})
	}
}

func TestRefreshRotation(t *testing.T) {
	e := newTestEnv(t)
	first := e.register("user@shop.test")

	var second authResponse
	e.expect(http.StatusOK, "POST", "/api/auth/refresh", "",
		map[string]any{"refreshToken": first.RefreshToken}).decode(t, &second)
	if second.RefreshToken == first.RefreshToken || second.AccessToken == "" {
		t.Fatal("refresh did not return a new token pair")
	}
	e.expect(http.StatusOK, "GET", "/api/auth/me", second.AccessToken, nil)

	// Reusing the rotated token signs out every session, including the new one.
	res := e.expect(http.StatusUnauthorized, "POST", "/api/auth/refresh", "",
		map[string]any{"refreshToken": first.RefreshToken})
	if !strings.Contains(res.errMsg(t), "already used") {
		t.Fatalf("unexpected error %s", res.raw)
	}
	e.expect(http.StatusUnauthorized, "POST", "/api/auth/refresh", "",
		map[string]any{"refreshToken": second.RefreshToken})

	e.expect(http.StatusBadRequest, "POST", "/api/auth/refresh", "", map[string]any{})
	e.expect(http.StatusUnauthorized, "POST", "/api/auth/refresh", "", map[string]any{"refreshToken": "nope"})
}

func TestRefreshTokenExpires(t *testing.T) {
	e := newTestEnv(t, func(c *config.Config) { c.RefreshTokenTTL = -time.Second })
	auth := e.register("user@shop.test")
	res := e.expect(http.StatusUnauthorized, "POST", "/api/auth/refresh", "",
		map[string]any{"refreshToken": auth.RefreshToken})
	if res.errMsg(t) != "Refresh token has expired" {
		t.Fatalf("unexpected error %s", res.raw)
	}
}

func TestConcurrentRefreshSucceedsOnce(t *testing.T) {
	e := newTestEnv(t)
	auth := e.register("user@shop.test")
	counts := parallel(t, 10, func() (response, error) {
		return e.send("POST", "/api/auth/refresh", "", map[string]any{"refreshToken": auth.RefreshToken})
	})
	if counts[http.StatusOK] != 1 || counts[http.StatusUnauthorized] != 9 {
		t.Fatalf("got status counts %v, want one 200 and nine 401", counts)
	}
}

func TestLogout(t *testing.T) {
	e := newTestEnv(t)
	e.register("user@shop.test")

	session := e.login("user@shop.test", userPassword)
	e.expect(http.StatusNoContent, "POST", "/api/auth/logout", "", map[string]any{"refreshToken": session.RefreshToken})
	res := e.expect(http.StatusUnauthorized, "POST", "/api/auth/refresh", "",
		map[string]any{"refreshToken": session.RefreshToken})
	if res.errMsg(t) != "Invalid refresh token" {
		t.Fatalf("logged-out token should be invalid, got %s", res.raw)
	}

	a := e.login("user@shop.test", userPassword)
	b := e.login("user@shop.test", userPassword)
	e.expect(http.StatusUnauthorized, "POST", "/api/auth/logout-all", "", nil)
	e.expect(http.StatusNoContent, "POST", "/api/auth/logout-all", a.AccessToken, nil)
	for _, s := range []authResponse{a, b} {
		e.expect(http.StatusUnauthorized, "POST", "/api/auth/refresh", "", map[string]any{"refreshToken": s.RefreshToken})
	}
}
