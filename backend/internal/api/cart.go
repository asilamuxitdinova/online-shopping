package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"online-shopping/internal/db"
)

type CartItem struct {
	ProductID int64   `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	ImageURL  *string `json:"image_url"`
	Stock     int64   `json:"stock"`
	Quantity  int64   `json:"quantity"`
	Subtotal  float64 `json:"subtotal"`
}

type Cart struct {
	Items []CartItem `json:"items"`
	Count int64      `json:"count"`
	Total float64    `json:"total"`
}

// loadCart returns the user's cart. With lock set, the cart rows stay locked
// until the transaction ends, so a concurrent checkout of the same cart waits.
func loadCart(ctx context.Context, q db.Querier, userID int64, lock bool) (Cart, error) {
	query := `SELECT p.id, p.name, p.price, p.image_url, p.stock, c.quantity
	          FROM cart_items c JOIN products p ON p.id = c.product_id
	          WHERE c.user_id = $1
	          ORDER BY p.name`
	if lock {
		query += " FOR UPDATE OF c"
	}
	rows, err := q.QueryContext(ctx, query, userID)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	cart := Cart{Items: []CartItem{}}
	var total float64
	for rows.Next() {
		var it CartItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Price, &it.ImageURL, &it.Stock, &it.Quantity); err != nil {
			return Cart{}, err
		}
		it.Subtotal = round2(it.Price * float64(it.Quantity))
		total += it.Price * float64(it.Quantity)
		cart.Count += it.Quantity
		cart.Items = append(cart.Items, it)
	}
	cart.Total = round2(total)
	return cart, rows.Err()
}

func productStock(ctx context.Context, q db.Querier, productID int64) (int64, error) {
	var stock int64
	err := q.QueryRowContext(ctx, `SELECT stock FROM products WHERE id = $1`, productID).Scan(&stock)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, httpErr(http.StatusNotFound, "Product not found")
	}
	return stock, err
}

var errBadQuantity = httpErr(http.StatusBadRequest, "Quantity must be a whole number from 1 to %d", maxCartQuantity)

func validQuantity(q int64) bool {
	return q >= 1 && q <= maxCartQuantity
}

// checkQuantity verifies a cart quantity against the product's stock and the per-product cap.
func checkQuantity(quantity, stock int64) error {
	if quantity > stock {
		return httpErr(http.StatusBadRequest, "Only %d item(s) in stock", stock)
	}
	if quantity > maxCartQuantity {
		return httpErr(http.StatusBadRequest, "At most %d of one product can be ordered", maxCartQuantity)
	}
	return nil
}

// GET /api/cart
func (s *Server) getCart(w http.ResponseWriter, r *http.Request) error {
	cart, err := loadCart(r.Context(), s.db, currentUser(r).ID, false)
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, cart)
}

// POST /api/cart/items  { productId, quantity = 1 }  — adds to the existing quantity
func (s *Server) addCartItem(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		ProductID int64  `json:"productId"`
		Quantity  *int64 `json:"quantity"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}
	add := int64(1)
	if body.Quantity != nil {
		add = *body.Quantity
	}
	if !validQuantity(add) {
		return errBadQuantity
	}

	ctx, userID := r.Context(), currentUser(r).ID
	var cart Cart
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		stock, err := productStock(ctx, tx, body.ProductID)
		if err != nil {
			return err
		}
		// Incrementing in SQL keeps concurrent adds from losing an update.
		var quantity int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO cart_items (user_id, product_id, quantity) VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, product_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
			 RETURNING quantity`,
			userID, body.ProductID, add,
		).Scan(&quantity); err != nil {
			return err
		}
		if err := checkQuantity(quantity, stock); err != nil {
			return err
		}
		cart, err = loadCart(ctx, tx, userID, false)
		return err
	})
	if err != nil {
		return err
	}
	return respond(w, http.StatusCreated, cart)
}

// PATCH /api/cart/items/{productId}  { quantity }  — sets the quantity
func (s *Server) updateCartItem(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Quantity *int64 `json:"quantity"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}
	if body.Quantity == nil || !validQuantity(*body.Quantity) {
		return errBadQuantity
	}

	ctx, userID, productID := r.Context(), currentUser(r).ID, pathID(r, "productId")
	var cart Cart
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE cart_items SET quantity = $3 WHERE user_id = $1 AND product_id = $2`,
			userID, productID, *body.Quantity)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return httpErr(http.StatusNotFound, "Item is not in the cart")
		}
		stock, err := productStock(ctx, tx, productID)
		if err != nil {
			return err
		}
		if err := checkQuantity(*body.Quantity, stock); err != nil {
			return err
		}
		cart, err = loadCart(ctx, tx, userID, false)
		return err
	})
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, cart)
}

// DELETE /api/cart/items/{productId}
func (s *Server) removeCartItem(w http.ResponseWriter, r *http.Request) error {
	ctx, userID := r.Context(), currentUser(r).ID
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM cart_items WHERE user_id = $1 AND product_id = $2`, userID, pathID(r, "productId"))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return httpErr(http.StatusNotFound, "Item is not in the cart")
	}

	cart, err := loadCart(ctx, s.db, userID, false)
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, cart)
}

// DELETE /api/cart
func (s *Server) clearCart(w http.ResponseWriter, r *http.Request) error {
	ctx, userID := r.Context(), currentUser(r).ID
	if _, err := s.db.ExecContext(ctx, `DELETE FROM cart_items WHERE user_id = $1`, userID); err != nil {
		return err
	}
	cart, err := loadCart(ctx, s.db, userID, false)
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, cart)
}
