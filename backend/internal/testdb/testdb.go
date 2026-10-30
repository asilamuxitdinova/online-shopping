// Package testdb gives each test its own empty, migrated PostgreSQL database.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"online-shopping/internal/db"
)

// New creates a fresh database for t, applies the migrations and returns a
// connection to it. The database is dropped when the test ends.
//
// The test is skipped unless TEST_DATABASE_URL points at a server where that
// user may create databases; `make test` starts a throwaway one.
func New(t *testing.T) *sql.DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set; run `make test` to use a throwaway PostgreSQL")
	}
	ctx := context.Background()

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	suffix := make([]byte, 6)
	rand.Read(suffix)
	name := "shop_test_" + hex.EncodeToString(suffix)
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("create test database: %v", err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	conn, err := db.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		if _, err := admin.ExecContext(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		admin.Close()
	})
	return conn
}
