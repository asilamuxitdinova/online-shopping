package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

func TestCart(t *testing.T) {
	e := newTestEnv(t)
	user := e.register("user@shop.test").AccessToken

	e.expect(http.StatusUnauthorized, "GET", "/api/cart", "", nil)

	cart := e.addToCart(user, 1, 2)
	if cart.Count != 2 || cart.Total != 179.98 || cart.Items[0].Subtotal != 179.98 {
		t.Fatalf("unexpected cart %+v", cart)
	}
	var added Cart
	e.expect(http.StatusCreated, "POST", "/api/cart/items", user, map[string]any{"productId": 1}).decode(t, &added)
	if added.Items[0].Quantity != 3 {
		t.Fatalf("adding again should increase quantity to 3, got %d", added.Items[0].Quantity)
	}

	// A rejected add must leave the cart unchanged.
	res := e.expect(http.StatusBadRequest, "POST", "/api/cart/items", user, map[string]any{"productId": 1, "quantity": 50})
	if res.errMsg(t) != "Only 25 item(s) in stock" {
		t.Fatalf("unexpected error %s", res.raw)
	}
	e.expect(http.StatusOK, "GET", "/api/cart", user, nil).decode(t, &cart)
	if cart.Items[0].Quantity != 3 {
		t.Fatalf("cart changed after rejected add: %+v", cart)
	}

	for _, q := range []int64{0, -1, maxCartQuantity + 1, 9999999999} {
		e.expect(http.StatusBadRequest, "POST", "/api/cart/items", user, map[string]any{"productId": 1, "quantity": q})
	}
	e.expect(http.StatusNotFound, "POST", "/api/cart/items", user, map[string]any{"productId": 999})

	e.expect(http.StatusNotFound, "PATCH", "/api/cart/items/2", user, map[string]any{"quantity": 1})
	e.expect(http.StatusBadRequest, "PATCH", "/api/cart/items/1", user, map[string]any{"quantity": 26})
	e.expect(http.StatusBadRequest, "PATCH", "/api/cart/items/1", user, map[string]any{})
	e.expect(http.StatusOK, "PATCH", "/api/cart/items/1", user, map[string]any{"quantity": 1}).decode(t, &cart)
	if cart.Count != 1 {
		t.Fatalf("patch should set quantity to 1, got %+v", cart)
	}

	e.addToCart(user, 2, 1)
	e.expect(http.StatusOK, "DELETE", "/api/cart/items/1", user, nil).decode(t, &cart)
	if len(cart.Items) != 1 || cart.Items[0].ProductID != 2 {
		t.Fatalf("unexpected cart after removing item: %+v", cart)
	}
	e.expect(http.StatusNotFound, "DELETE", "/api/cart/items/1", user, nil)
	e.expect(http.StatusOK, "DELETE", "/api/cart", user, nil).decode(t, &cart)
	if len(cart.Items) != 0 || cart.Total != 0 {
		t.Fatalf("cart should be empty: %+v", cart)
	}
}

func TestCartQuantityCap(t *testing.T) {
	e := newTestEnv(t)
	user := e.register("user@shop.test").AccessToken
	p := e.createProduct(e.adminToken(), map[string]any{"name": "Pen", "price": 1, "stock": 5000})

	e.addToCart(user, p.ID, maxCartQuantity)
	res := e.expect(http.StatusBadRequest, "POST", "/api/cart/items", user, map[string]any{"productId": p.ID})
	if !strings.HasPrefix(res.errMsg(t), "At most") {
		t.Fatalf("unexpected error %s", res.raw)
	}
}

func TestCartsAreSeparate(t *testing.T) {
	e := newTestEnv(t)
	a := e.register("a@shop.test").AccessToken
	b := e.register("b@shop.test").AccessToken

	e.addToCart(a, 1, 1)
	var cart Cart
	e.expect(http.StatusOK, "GET", "/api/cart", b, nil).decode(t, &cart)
	if len(cart.Items) != 0 {
		t.Fatalf("user b sees user a's cart: %+v", cart)
	}
}
