package seed

import (
	"context"
	"database/sql"
	"testing"

	"online-shopping/internal/config"
	"online-shopping/internal/testdb"
)

func TestCheckAdminPassword(t *testing.T) {
	for password, ok := range map[string]bool{
		"change-me":         false,
		"admin12345":        false,
		"short-pass1":       false,
		"a-strong-password": true,
	} {
		if err := checkAdminPassword(password); (err == nil) != ok {
			t.Errorf("checkAdminPassword(%q) = %v", password, err)
		}
	}
}

func count(t *testing.T, conn *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunIsIdempotent(t *testing.T) {
	conn := testdb.New(t)
	cfg := &config.Config{AdminEmail: "Admin@Shop.test", AdminPassword: "a-strong-password"}
	for range 2 {
		if err := Run(context.Background(), conn, cfg); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, conn, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND email = 'admin@shop.test'`); n != 1 {
		t.Fatalf("got %d admins, want 1", n)
	}
	if n := count(t, conn, `SELECT COUNT(*) FROM products`); n != len(sampleProducts) {
		t.Fatalf("got %d products, want %d", n, len(sampleProducts))
	}
}

func TestRunRejectsWeakAdminPassword(t *testing.T) {
	conn := testdb.New(t)
	cfg := &config.Config{AdminEmail: "admin@shop.test", AdminPassword: "admin12345"}
	if err := Run(context.Background(), conn, cfg); err == nil {
		t.Fatal("seed accepted the example admin password")
	}
	if n := count(t, conn, `SELECT COUNT(*) FROM users`) + count(t, conn, `SELECT COUNT(*) FROM products`); n != 0 {
		t.Fatal("seed wrote data despite rejecting the password")
	}
}
