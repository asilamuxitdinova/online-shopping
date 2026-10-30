package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Product struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int64     `json:"stock"`
	Category    string    `json:"category"`
	ImageURL    *string   `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const productColumns = `id, name, description, price, stock, category, image_url, created_at, updated_at`

var productSorts = map[string]string{
	"newest":     "created_at DESC, id DESC",
	"price_asc":  "price ASC, id",
	"price_desc": "price DESC, id",
	"name":       "lower(name) ASC, id",
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(row scanner) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Server) findProduct(ctx context.Context, id int64) (Product, error) {
	p, err := scanProduct(s.db.QueryRowContext(ctx, "SELECT "+productColumns+" FROM products WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, httpErr(http.StatusNotFound, "Product not found")
	}
	return p, err
}

// queryArgs collects query arguments and hands out their $n placeholders.
type queryArgs []any

func (a *queryArgs) add(v any) string {
	*a = append(*a, v)
	return "$" + strconv.Itoa(len(*a))
}

type productInput struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	Stock       *int64   `json:"stock"`
	Category    *string  `json:"category"`
	ImageURL    *string  `json:"image_url"`
}

// columns validates the input and returns the columns and values to write.
// Unless partial is set, name and price are required.
func (in productInput) columns(partial bool) (cols []string, vals []any, err error) {
	add := func(col string, val any) {
		cols = append(cols, col)
		vals = append(vals, val)
	}

	if in.Name != nil || !partial {
		var name string
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		if name == "" {
			return nil, nil, httpErr(http.StatusBadRequest, "Name is required")
		}
		if tooLong(name, maxNameLen) {
			return nil, nil, httpErr(http.StatusBadRequest, "Name must be at most %d characters", maxNameLen)
		}
		add("name", name)
	}
	if in.Description != nil {
		if tooLong(*in.Description, maxDescriptionLen) {
			return nil, nil, httpErr(http.StatusBadRequest, "Description must be at most %d characters", maxDescriptionLen)
		}
		add("description", *in.Description)
	}
	if in.Price != nil || !partial {
		if in.Price == nil || *in.Price < 0 || *in.Price > maxPrice {
			return nil, nil, httpErr(http.StatusBadRequest, "Price must be a number from 0 to %d", maxPrice)
		}
		add("price", round2(*in.Price))
	}
	if in.Stock != nil {
		if *in.Stock < 0 || *in.Stock > maxStock {
			return nil, nil, httpErr(http.StatusBadRequest, "Stock must be a whole number from 0 to %d", maxStock)
		}
		add("stock", *in.Stock)
	}
	if in.Category != nil {
		category := strings.ToLower(strings.TrimSpace(*in.Category))
		if category == "" {
			category = "general"
		}
		if tooLong(category, maxNameLen) {
			return nil, nil, httpErr(http.StatusBadRequest, "Category must be at most %d characters", maxNameLen)
		}
		add("category", category)
	}
	if in.ImageURL != nil {
		var url any // an empty string clears the image
		if *in.ImageURL != "" {
			if !validImageURL(*in.ImageURL) {
				return nil, nil, httpErr(http.StatusBadRequest, "Image URL must be an http or https link")
			}
			url = *in.ImageURL
		}
		add("image_url", url)
	}
	return cols, vals, nil
}

// GET /api/products?search=&category=&minPrice=&maxPrice=&sort=&page=&limit=
func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	var where []string
	var args queryArgs

	if v := q.Get("search"); v != "" {
		p := args.add("%" + likeEscaper.Replace(v) + "%")
		where = append(where, fmt.Sprintf("(name ILIKE %s OR description ILIKE %s)", p, p))
	}
	if v := q.Get("category"); v != "" {
		where = append(where, "category = "+args.add(strings.ToLower(v)))
	}
	if q.Has("minPrice") {
		price, _ := strconv.ParseFloat(q.Get("minPrice"), 64)
		where = append(where, "price >= "+args.add(price))
	}
	if q.Has("maxPrice") {
		price, _ := strconv.ParseFloat(q.Get("maxPrice"), 64)
		where = append(where, "price <= "+args.add(price))
	}

	orderBy, ok := productSorts[q.Get("sort")]
	if !ok {
		orderBy = productSorts["newest"]
	}

	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		limit = 20
	}
	limit = min(max(limit, 1), 100)
	page, err := strconv.Atoi(q.Get("page"))
	if err != nil {
		page = 1
	}
	page = max(page, 1)

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM products "+whereSQL, args...).Scan(&total); err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT %s FROM products %s ORDER BY %s LIMIT %s OFFSET %s",
		productColumns, whereSQL, orderBy, args.add(limit), args.add((page-1)*limit))
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	items := []Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return respond(w, http.StatusOK, map[string]any{
		"items": items,
		"page":  page,
		"limit": limit,
		"total": total,
		"pages": (total + limit - 1) / limit,
	})
}

// GET /api/products/categories
func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) error {
	type categoryCount struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	}

	rows, err := s.db.QueryContext(r.Context(),
		`SELECT category, COUNT(*) FROM products GROUP BY category ORDER BY category`)
	if err != nil {
		return err
	}
	defer rows.Close()

	categories := []categoryCount{}
	for rows.Next() {
		var c categoryCount
		if err := rows.Scan(&c.Category, &c.Count); err != nil {
			return err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return respond(w, http.StatusOK, map[string]any{"categories": categories})
}

// GET /api/products/{id}
func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) error {
	p, err := s.findProduct(r.Context(), pathID(r, "id"))
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, map[string]Product{"product": p})
}

// POST /api/products  (admin)
func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) error {
	var in productInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	cols, vals, err := in.columns(false)
	if err != nil {
		return err
	}

	var args queryArgs
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		placeholders[i] = args.add(v)
	}
	query := fmt.Sprintf("INSERT INTO products (%s) VALUES (%s) RETURNING %s",
		strings.Join(cols, ", "), strings.Join(placeholders, ", "), productColumns)
	p, err := scanProduct(s.db.QueryRowContext(r.Context(), query, args...))
	if err != nil {
		return err
	}
	return respond(w, http.StatusCreated, map[string]Product{"product": p})
}

// PATCH /api/products/{id}  (admin)
func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) error {
	id := pathID(r, "id")
	var in productInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if _, err := s.findProduct(r.Context(), id); err != nil {
		return err
	}
	cols, vals, err := in.columns(true)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return httpErr(http.StatusBadRequest, "No fields to update")
	}

	var args queryArgs
	set := make([]string, len(cols))
	for i, c := range cols {
		set[i] = c + " = " + args.add(vals[i])
	}
	query := fmt.Sprintf("UPDATE products SET %s, updated_at = now() WHERE id = %s RETURNING %s",
		strings.Join(set, ", "), args.add(id), productColumns)
	p, err := scanProduct(s.db.QueryRowContext(r.Context(), query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return httpErr(http.StatusNotFound, "Product not found")
	}
	if err != nil {
		return err
	}
	return respond(w, http.StatusOK, map[string]Product{"product": p})
}

// DELETE /api/products/{id}  (admin)
func (s *Server) deleteProduct(w http.ResponseWriter, r *http.Request) error {
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM products WHERE id = $1`, pathID(r, "id"))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return httpErr(http.StatusNotFound, "Product not found")
	}
	return respond(w, http.StatusNoContent, nil)
}
