// Package seed creates the admin account (from .env) and some sample products.
package seed

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"online-shopping/internal/config"
	"online-shopping/internal/db"
)

var sampleProducts = []struct {
	name, description string
	price             float64
	stock             int
	category          string
}{
	{"Wireless Headphones", "Noise-cancelling over-ear headphones", 89.99, 25, "electronics"},
	{"Smartphone Stand", "Adjustable aluminium phone stand", 14.5, 100, "electronics"},
	{"Cotton T-Shirt", "100% cotton, unisex fit", 12.0, 200, "clothing"},
	{"Denim Jacket", "Classic blue denim jacket", 59.0, 40, "clothing"},
	{"Ceramic Mug", "350 ml, dishwasher safe", 8.75, 150, "home"},
	{"Desk Lamp", "LED lamp with three brightness levels", 27.3, 60, "home"},
}

// Passwords that ship in example files and must never protect a real admin account.
var placeholderPasswords = []string{"change-me", "admin12345"}

const minAdminPasswordLen = 12

// checkAdminPassword rejects placeholder and short admin passwords.
func checkAdminPassword(password string) error {
	if slices.Contains(placeholderPasswords, password) || len(password) < minAdminPasswordLen {
		return fmt.Errorf("ADMIN_PASSWORD must be at least %d characters and not an example value; "+
			"set a strong one in .env", minAdminPasswordLen)
	}
	return nil
}

// Run is idempotent: it only adds the admin if missing and products if the table is empty.
func Run(ctx context.Context, conn *sql.DB, cfg *config.Config) error {
	if cfg.AdminEmail != "" {
		if err := checkAdminPassword(cfg.AdminPassword); err != nil {
			return err
		}
	}

	err := db.WithTx(ctx, conn, func(tx *sql.Tx) error {
		if cfg.AdminEmail != "" {
			email := strings.ToLower(cfg.AdminEmail)
			var exists bool
			if err := tx.QueryRowContext(ctx,
				`SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
				if err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO users (name, email, password_hash, role) VALUES ('Admin', $1, $2, 'admin')`,
					email, string(hash)); err != nil {
					return err
				}
				log.Printf("Created admin user %s", email)
			}
		}

		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			for _, p := range sampleProducts {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO products (name, description, price, stock, category) VALUES ($1, $2, $3, $4, $5)`,
					p.name, p.description, p.price, p.stock, p.category); err != nil {
					return err
				}
			}
			log.Printf("Inserted %d sample products", len(sampleProducts))
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Println("Seed complete")
	return nil
}
