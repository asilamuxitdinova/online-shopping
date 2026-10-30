// Package config loads application settings from the environment and .env.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// jwtSecretPlaceholder is the value shipped in .env.example; it must never be used.
const jwtSecretPlaceholder = "change-me-to-a-long-random-string"

type Config struct {
	Host              string // address to listen on; 127.0.0.1 when behind nginx
	Port              string
	JWTSecret         []byte
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	DatabaseURL       string
	CORSOrigins       []string // browser origins allowed to call the API; "*" allows any
	TrustProxy        bool     // take the client IP from X-Real-IP, set by a reverse proxy
	AuthRatePerMinute int      // login/register/refresh attempts allowed per client IP
	AdminEmail        string
	AdminPassword     string
}

// Load reads settings from the environment after loading .env, if present.
// Variables already set in the environment take precedence over .env.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 || secret == jwtSecretPlaceholder {
		return nil, errors.New("JWT_SECRET must be a random string of at least 32 characters " +
			"(generate one with: openssl rand -hex 32)")
	}

	accessTTL, err := parseDuration(getenv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("ACCESS_TOKEN_TTL: %w", err)
	}
	refreshTTL, err := parseDuration(getenv("REFRESH_TOKEN_TTL", "30d"))
	if err != nil {
		return nil, fmt.Errorf("REFRESH_TOKEN_TTL: %w", err)
	}
	trustProxy, err := strconv.ParseBool(getenv("TRUST_PROXY", "false"))
	if err != nil {
		return nil, fmt.Errorf("TRUST_PROXY: %w", err)
	}
	authRate, err := strconv.Atoi(getenv("AUTH_RATE_PER_MINUTE", "10"))
	if err != nil || authRate < 1 {
		return nil, errors.New("AUTH_RATE_PER_MINUTE must be a positive integer")
	}

	return &Config{
		Host:              getenv("HOST", "127.0.0.1"),
		Port:              getenv("PORT", "5000"),
		JWTSecret:         []byte(secret),
		AccessTokenTTL:    accessTTL,
		RefreshTokenTTL:   refreshTTL,
		DatabaseURL:       databaseURL(),
		CORSOrigins:       splitList(os.Getenv("CORS_ORIGINS")),
		TrustProxy:        trustProxy,
		AuthRatePerMinute: authRate,
		AdminEmail:        os.Getenv("ADMIN_EMAIL"),
		AdminPassword:     os.Getenv("ADMIN_PASSWORD"),
	}, nil
}

// databaseURL returns DATABASE_URL if set, otherwise builds one from the DB_* variables.
func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(getenv("DB_USER", "shop"), os.Getenv("DB_PASSWORD")),
		Host:     net.JoinHostPort(getenv("DB_HOST", "localhost"), getenv("DB_PORT", "5432")),
		Path:     "/" + getenv("DB_NAME", "shop"),
		RawQuery: "sslmode=" + getenv("DB_SSLMODE", "disable"),
	}
	return u.String()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitList splits a comma-separated value, dropping empty entries.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseDuration accepts Go durations ("12h", "30m") and whole days ("7d").
func parseDuration(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
