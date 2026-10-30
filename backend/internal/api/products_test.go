package api

import (
	"net/http"
	"strings"
	"testing"
)

type productPage struct {
	Items []Product
	Page  int
	Limit int
	Total int
	Pages int
}

func TestListProducts(t *testing.T) {
	e := newTestEnv(t) // seeded with 6 products in 3 categories

	tests := []struct {
		query string
		total int
		first string
	}{
		{"", 6, ""},
		{"?category=HOME", 2, ""},
		{"?search=HEADPHONES", 1, "Wireless Headphones"},
		{"?search=%25", 1, "Cotton T-Shirt"}, // "%" matches literally ("100% cotton"), not everything
		{"?search=_", 0, ""},
		{"?minPrice=100000&maxPrice=400000", 3, ""},
		{"?sort=price_asc", 6, "Ceramic Mug"},
		{"?sort=price_desc", 6, "Wireless Headphones"},
		{"?sort=name", 6, "Ceramic Mug"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			var page productPage
			e.expect(http.StatusOK, "GET", "/api/products"+tt.query, "", nil).decode(t, &page)
			if page.Total != tt.total || len(page.Items) != tt.total {
				t.Fatalf("got total %d with %d items, want %d", page.Total, len(page.Items), tt.total)
			}
			if tt.first != "" && page.Items[0].Name != tt.first {
				t.Fatalf("first item %q, want %q", page.Items[0].Name, tt.first)
			}
		})
	}

	var page productPage
	e.expect(http.StatusOK, "GET", "/api/products?sort=name&limit=2&page=2", "", nil).decode(t, &page)
	if page.Pages != 3 || len(page.Items) != 2 || page.Items[0].Name != "Denim Jacket" || page.Items[1].Name != "Desk Lamp" {
		t.Fatalf("unexpected page %+v", page)
	}

	var cats struct{ Categories []struct{ Category string } }
	e.expect(http.StatusOK, "GET", "/api/products/categories", "", nil).decode(t, &cats)
	if len(cats.Categories) != 3 {
		t.Fatalf("got %d categories, want 3", len(cats.Categories))
	}

	if p := e.product(1); p.Name != "Wireless Headphones" || p.Price != 1_150_000 {
		t.Fatalf("unexpected product %+v", p)
	}
	e.expect(http.StatusNotFound, "GET", "/api/products/999", "", nil)
	e.expect(http.StatusNotFound, "GET", "/api/products/abc", "", nil)
}

func TestProductValidation(t *testing.T) {
	e := newTestEnv(t)
	admin := e.adminToken()

	tests := []struct {
		name string
		body any
		want string
	}{
		{"missing name", map[string]any{"price": 1}, "Name is required"},
		{"long name", map[string]any{"name": strings.Repeat("x", maxNameLen+1), "price": 1}, "Name must be"},
		{"missing price", map[string]any{"name": "x"}, "Price must be"},
		{"negative price", map[string]any{"name": "x", "price": -1}, "Price must be"},
		{"huge price", map[string]any{"name": "x", "price": 1e12}, "Price must be"},
		{"negative stock", map[string]any{"name": "x", "price": 1, "stock": -1}, "Stock must be"},
		{"huge stock", map[string]any{"name": "x", "price": 1, "stock": 9999999999}, "Stock must be"},
		{"fractional stock", map[string]any{"name": "x", "price": 1, "stock": 1.5}, "Invalid JSON body"},
		{"javascript image", map[string]any{"name": "x", "price": 1, "image_url": "javascript:alert(1)"}, "Image URL"},
		{"relative image", map[string]any{"name": "x", "price": 1, "image_url": "/img.png"}, "Image URL"},
		{"long description", map[string]any{"name": "x", "price": 1, "description": strings.Repeat("d", maxDescriptionLen+1)}, "Description"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := e.do("POST", "/api/products", admin, tt.body)
			if res.status != http.StatusBadRequest || !strings.Contains(res.errMsg(t), tt.want) {
				t.Fatalf("got %d %s, want 400 containing %q", res.status, res.raw, tt.want)
			}
		})
	}
}

func TestProductAdmin(t *testing.T) {
	e := newTestEnv(t)
	admin := e.adminToken()
	customer := e.register("user@shop.test").AccessToken
	body := map[string]any{"name": "Backpack", "price": 35.499, "stock": 10, "category": " Bags "}

	e.expect(http.StatusUnauthorized, "POST", "/api/products", "", body)
	e.expect(http.StatusForbidden, "POST", "/api/products", customer, body)

	p := e.createProduct(admin, body)
	if p.Price != 35.5 || p.Category != "bags" || p.Stock != 10 || p.ImageURL != nil {
		t.Fatalf("unexpected product %+v", p)
	}

	var updated struct{ Product Product }
	e.expect(http.StatusOK, "PATCH", "/api/products/"+itoa(p.ID), admin,
		map[string]any{"price": 30, "image_url": "https://cdn.shop.test/bag.png"}).decode(t, &updated)
	if updated.Product.Price != 30 || updated.Product.ImageURL == nil || updated.Product.Name != "Backpack" {
		t.Fatalf("unexpected update %+v", updated.Product)
	}
	e.expect(http.StatusOK, "PATCH", "/api/products/"+itoa(p.ID), admin, map[string]any{"image_url": ""}).decode(t, &updated)
	if updated.Product.ImageURL != nil {
		t.Fatal("empty image_url should clear the image")
	}

	e.expect(http.StatusBadRequest, "PATCH", "/api/products/"+itoa(p.ID), admin, map[string]any{})
	e.expect(http.StatusNotFound, "PATCH", "/api/products/999", admin, map[string]any{"price": 1})
	e.expect(http.StatusForbidden, "DELETE", "/api/products/"+itoa(p.ID), customer, nil)

	e.expect(http.StatusNoContent, "DELETE", "/api/products/"+itoa(p.ID), admin, nil)
	e.expect(http.StatusNotFound, "GET", "/api/products/"+itoa(p.ID), "", nil)
	e.expect(http.StatusNotFound, "DELETE", "/api/products/"+itoa(p.ID), admin, nil)
}
