package config

import (
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

var envKeys = []string{
	"HOST", "PORT", "JWT_SECRET", "ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "TRUST_PROXY",
	"AUTH_RATE_PER_MINUTE", "CORS_ORIGINS", "DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER",
	"DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "ADMIN_EMAIL", "ADMIN_PASSWORD",
}

var goodSecret = strings.Repeat("a", 32)

// setEnv clears every setting, then applies vars for the duration of the test.
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range envKeys {
		t.Setenv(k, "")
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoadRejectsWeakSecret(t *testing.T) {
	for _, secret := range []string{"", jwtSecretPlaceholder, strings.Repeat("a", 31)} {
		setEnv(t, map[string]string{"JWT_SECRET": secret})
		if _, err := Load(); err == nil {
			t.Errorf("JWT_SECRET %q was accepted", secret)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, map[string]string{"JWT_SECRET": goodSecret})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != "5000" || cfg.TrustProxy || cfg.AuthRatePerMinute != 10 ||
		cfg.AccessTokenTTL != 15*time.Minute || cfg.RefreshTokenTTL != 30*24*time.Hour || cfg.CORSOrigins != nil {
		t.Fatalf("unexpected defaults %+v", cfg)
	}
	if want := "postgres://shop:@localhost:5432/shop?sslmode=disable"; cfg.DatabaseURL != want {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

func TestLoadValues(t *testing.T) {
	setEnv(t, map[string]string{
		"JWT_SECRET":           goodSecret,
		"HOST":                 "0.0.0.0",
		"ACCESS_TOKEN_TTL":     "5m",
		"REFRESH_TOKEN_TTL":    "7d",
		"TRUST_PROXY":          "true",
		"AUTH_RATE_PER_MINUTE": "30",
		"CORS_ORIGINS":         " https://a.test , ,https://b.test",
		"DB_HOST":              "db.internal",
		"DB_PASSWORD":          "p@ss:w/rd?#",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "0.0.0.0" || !cfg.TrustProxy || cfg.AuthRatePerMinute != 30 ||
		cfg.AccessTokenTTL != 5*time.Minute || cfg.RefreshTokenTTL != 7*24*time.Hour {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if !slices.Equal(cfg.CORSOrigins, []string{"https://a.test", "https://b.test"}) {
		t.Fatalf("CORSOrigins = %q", cfg.CORSOrigins)
	}

	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if pw, _ := u.User.Password(); pw != "p@ss:w/rd?#" || u.Hostname() != "db.internal" {
		t.Fatalf("password or host lost in %q", cfg.DatabaseURL)
	}

	t.Setenv("DATABASE_URL", "postgres://u:p@elsewhere/db")
	if cfg, _ := Load(); cfg.DatabaseURL != "postgres://u:p@elsewhere/db" {
		t.Fatalf("DATABASE_URL should take precedence, got %q", cfg.DatabaseURL)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	for key, value := range map[string]string{
		"ACCESS_TOKEN_TTL":     "soon",
		"REFRESH_TOKEN_TTL":    "0d",
		"TRUST_PROXY":          "maybe",
		"AUTH_RATE_PER_MINUTE": "0",
	} {
		setEnv(t, map[string]string{"JWT_SECRET": goodSecret, key: value})
		if _, err := Load(); err == nil {
			t.Errorf("%s=%q was accepted", key, value)
		}
	}
}
