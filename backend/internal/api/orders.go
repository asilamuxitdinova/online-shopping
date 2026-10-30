package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"online-shopping/internal/db"
)

var orderStatuses = []string{"pending", "paid", "shipped", "delivered", "cancelled"}

type OrderItem struct {
	ProductID *int64  `json:"product_id"` // nil once the product has been deleted
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int64   `json:"quantity"`
	Subtotal  float64 `json:"subtotal"`
}

type Order struct {
	ID              int64       `json:"id"`
	UserID          int64       `json:"user_id"`
	Status          string      `json:"status"`
	Total           float64     `json:"total"`
	ShippingAddress string      `json:"shipping_address"`
	CreatedAt       time.Time   `json:"created_at"`
	Items           []OrderItem `json:"items"`
}

const orderColumns = `id, user_id, status, total, shipping_address, created_at`

func scanOrder(row scanner) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.ShippingAddress, &o.CreatedAt)
	return o, err
}

func loadOrderItems(ctx context.Context, q db.Querier, o *Order) error {
	rows, err := q.QueryContext(ctx,
		`SELECT product_id, name, price, quantity FROM order_items WHERE order_id = $1 ORDER BY id`, o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	o.Items = []OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Price, &it.Quantity); err != nil {
			return err
		}
		it.Subtotal = round2(it.Price * float64(it.Quantity))
		o.Items = append(o.Items, it)
	}
	return rows.Err()
}

// loadOrder fetches an order with its items if the user may see it: their own, or any order
// for admins. With lock set, the order row stays locked until the transaction ends.
func loadOrder(ctx context.Context, q db.Querier, id int64, user User, lock bool) (Order, error) {
	query := "SELECT " + orderColumns + " FROM orders WHERE id = $1"
	if lock {
		query += " FOR UPDATE"
	}
	o, err := scanOrder(q.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && o.UserID != user.ID && user.Role != "admin") {
		return Order{}, httpErr(http.StatusNotFound, "Order not found")
	}
	if err != nil {
		return Order{}, err
	}
	return o, loadOrderItems(ctx, q, &o)
}

// setOrderStatus updates the status, returning items to stock when the order is cancelled.
// The order must have been loaded with lock set in the same transaction.
func setOrderStatus(ctx context.Context, tx *sql.Tx, o Order, status string) error {
	if status == "cancelled" {
		for _, it := range o.Items {
			if it.ProductID == nil {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE products SET stock = stock + $1 WHERE id = $2`, it.Quantity, *it.ProductID); err != nil {
				return err
			}
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE orders SET status = $1 WHERE id = $2`, status, o.ID)
	return err
}

// POST /api/orders  { shippingAddress }  — checks out the current cart
func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		ShippingAddress string `json:"shippingAddress"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}
	address := strings.TrimSpace(body.ShippingAddress)
	if address == "" {
		return httpErr(http.StatusBadRequest, "Shipping address is required")
	}
	if tooLong(address, maxAddressLen) {
		return httpErr(http.StatusBadRequest, "Shipping address must be at most %d characters", maxAddressLen)
	}

	ctx, user := r.Context(), currentUser(r)
	var order Order
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		cart, err := loadCart(ctx, tx, user.ID, true)
		if err != nil {
			return err
		}
		if len(cart.Items) == 0 {
			return httpErr(http.StatusBadRequest, "Cart is empty")
		}

		for _, it := range cart.Items {
			res, err := tx.ExecContext(ctx,
				`UPDATE products SET stock = stock - $1 WHERE id = $2 AND stock >= $1`,
				it.Quantity, it.ProductID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return httpErr(http.StatusConflict, "Not enough stock for %q", it.Name)
			}
		}

		var id int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO orders (user_id, total, shipping_address) VALUES ($1, $2, $3) RETURNING id`,
			user.ID, cart.Total, address,
		).Scan(&id); err != nil {
			return err
		}
		for _, it := range cart.Items {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, product_id, name, price, quantity) VALUES ($1, $2, $3, $4, $5)`,
				id, it.ProductID, it.Name, it.Price, it.Quantity); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE user_id = $1`, user.ID); err != nil {
			return err
		}

		order, err = loadOrder(ctx, tx, id, user, false)
		return err
	})
	if err != nil {
		return err
	}
	return respond(w, http.StatusCreated, map[string]Order{"order": order})
}

// GET /api/orders  — own orders; admins can pass ?all=true to see everyone's
func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) error {
	ctx, user := r.Context(), currentUser(r)

	query := "SELECT " + orderColumns + " FROM orders WHERE user_id = $1 ORDER BY id DESC"
	args := []any{user.ID}
	if user.Role == "admin" && r.URL.Query().Get("all") == "true" {
		query = "SELECT " + orderColumns + " FROM orders ORDER BY id DESC"
		args = nil
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range orders {
		if err := loadOrderItems(ctx, s.db, &orders[i]); err != nil {
			return err
		}
	}
	return respond(w, http.StatusOK, map[string][]Order{"orders": orders})
}

// GET /api/orders/{id}
func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) error {
	order, err := loadOrder(r.Context(), s.db, pathID(r, "id"), currentUser(r), false)
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, map[string]Order{"order": order})
}

// POST /api/orders/{id}/cancel  — only pending orders; stock is returned
func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) error {
	ctx, user, id := r.Context(), currentUser(r), pathID(r, "id")
	var order Order
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		o, err := loadOrder(ctx, tx, id, user, true)
		if err != nil {
			return err
		}
		if o.Status != "pending" {
			return httpErr(http.StatusBadRequest, "Cannot cancel an order that is %s", o.Status)
		}
		if err := setOrderStatus(ctx, tx, o, "cancelled"); err != nil {
			return err
		}
		order, err = loadOrder(ctx, tx, id, user, false)
		return err
	})
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, map[string]Order{"order": order})
}

// PATCH /api/orders/{id}/status  { status }  (admin)
func (s *Server) updateOrderStatus(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return err
	}

	ctx, user, id := r.Context(), currentUser(r), pathID(r, "id")
	var order Order
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		o, err := loadOrder(ctx, tx, id, user, true)
		if err != nil {
			return err
		}
		if !slices.Contains(orderStatuses, body.Status) {
			return httpErr(http.StatusBadRequest, "Status must be one of: %s", strings.Join(orderStatuses, ", "))
		}
		if o.Status == "cancelled" {
			return httpErr(http.StatusBadRequest, "Order is already cancelled")
		}
		if err := setOrderStatus(ctx, tx, o, body.Status); err != nil {
			return err
		}
		order, err = loadOrder(ctx, tx, id, user, false)
		return err
	})
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, map[string]Order{"order": order})
}
