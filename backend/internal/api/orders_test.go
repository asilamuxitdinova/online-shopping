package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestCheckout(t *testing.T) {
	e := newTestEnv(t)
	user := e.register("user@shop.test").AccessToken

	e.expect(http.StatusBadRequest, "POST", "/api/orders", user, map[string]any{"shippingAddress": "Tashkent"}) // empty cart
	e.addToCart(user, 1, 3)
	e.expect(http.StatusBadRequest, "POST", "/api/orders", user, map[string]any{"shippingAddress": "  "})
	e.expect(http.StatusBadRequest, "POST", "/api/orders", user,
		map[string]any{"shippingAddress": strings.Repeat("a", maxAddressLen+1)})

	order := e.checkout(user)
	if order.Status != "pending" || order.Total != 269.97 || len(order.Items) != 1 || order.Items[0].Quantity != 3 {
		t.Fatalf("unexpected order %+v", order)
	}
	if stock := e.product(1).Stock; stock != 22 {
		t.Fatalf("stock after checkout = %d, want 22", stock)
	}
	var cart Cart
	e.expect(http.StatusOK, "GET", "/api/cart", user, nil).decode(t, &cart)
	if len(cart.Items) != 0 {
		t.Fatal("cart should be empty after checkout")
	}

	var list struct{ Orders []Order }
	e.expect(http.StatusOK, "GET", "/api/orders", user, nil).decode(t, &list)
	if len(list.Orders) != 1 || list.Orders[0].ID != order.ID || len(list.Orders[0].Items) != 1 {
		t.Fatalf("unexpected order list %+v", list)
	}
	e.expect(http.StatusOK, "GET", "/api/orders/"+itoa(order.ID), user, nil)
}

func TestCheckoutStockConflict(t *testing.T) {
	e := newTestEnv(t)
	p := e.createProduct(e.adminToken(), map[string]any{"name": "Last one", "price": 5, "stock": 2})
	a := e.register("a@shop.test").AccessToken
	b := e.register("b@shop.test").AccessToken

	e.addToCart(a, p.ID, 2)
	e.addToCart(b, p.ID, 2)
	e.checkout(a)
	res := e.expect(http.StatusConflict, "POST", "/api/orders", b, map[string]any{"shippingAddress": "Tashkent"})
	if res.errMsg(t) != `Not enough stock for "Last one"` {
		t.Fatalf("unexpected error %s", res.raw)
	}
	if stock := e.product(p.ID).Stock; stock != 0 {
		t.Fatalf("stock = %d, want 0", stock)
	}
}

func TestOrderAccessAndCancel(t *testing.T) {
	e := newTestEnv(t)
	owner := e.register("owner@shop.test").AccessToken
	other := e.register("other@shop.test").AccessToken

	e.addToCart(owner, 1, 3)
	order := e.checkout(owner)
	path := "/api/orders/" + itoa(order.ID)

	e.expect(http.StatusNotFound, "GET", path, other, nil)
	e.expect(http.StatusNotFound, "POST", path+"/cancel", other, nil)
	e.expect(http.StatusNotFound, "GET", "/api/orders/999", owner, nil)

	var cancelled struct{ Order Order }
	e.expect(http.StatusOK, "POST", path+"/cancel", owner, nil).decode(t, &cancelled)
	if cancelled.Order.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", cancelled.Order.Status)
	}
	if stock := e.product(1).Stock; stock != 25 {
		t.Fatalf("stock after cancel = %d, want 25", stock)
	}
	res := e.expect(http.StatusBadRequest, "POST", path+"/cancel", owner, nil)
	if res.errMsg(t) != "Cannot cancel an order that is cancelled" {
		t.Fatalf("unexpected error %s", res.raw)
	}
}

func TestAdminOrderStatus(t *testing.T) {
	e := newTestEnv(t)
	admin := e.adminToken()
	user := e.register("user@shop.test").AccessToken

	e.addToCart(user, 1, 2)
	order := e.checkout(user)
	path := "/api/orders/" + itoa(order.ID)

	e.expect(http.StatusForbidden, "PATCH", path+"/status", user, map[string]any{"status": "paid"})
	e.expect(http.StatusBadRequest, "PATCH", path+"/status", admin, map[string]any{"status": "lost"})
	e.expect(http.StatusNotFound, "PATCH", "/api/orders/999/status", admin, map[string]any{"status": "paid"})

	var updated struct{ Order Order }
	e.expect(http.StatusOK, "PATCH", path+"/status", admin, map[string]any{"status": "shipped"}).decode(t, &updated)
	if updated.Order.Status != "shipped" {
		t.Fatalf("status = %q, want shipped", updated.Order.Status)
	}
	e.expect(http.StatusBadRequest, "POST", path+"/cancel", user, nil) // customers can only cancel pending orders

	e.expect(http.StatusOK, "PATCH", path+"/status", admin, map[string]any{"status": "cancelled"})
	if stock := e.product(1).Stock; stock != 25 {
		t.Fatalf("admin cancel should restock: stock = %d, want 25", stock)
	}
	e.expect(http.StatusBadRequest, "PATCH", path+"/status", admin, map[string]any{"status": "paid"})

	var all, own struct{ Orders []Order }
	e.addToCart(admin, 2, 1)
	e.checkout(admin)
	e.expect(http.StatusOK, "GET", "/api/orders?all=true", admin, nil).decode(t, &all)
	e.expect(http.StatusOK, "GET", "/api/orders?all=true", user, nil).decode(t, &own)
	if len(all.Orders) != 2 || len(own.Orders) != 1 {
		t.Fatalf("admin sees %d orders (want 2), customer sees %d (want 1)", len(all.Orders), len(own.Orders))
	}
}

func TestOrderKeepsDeletedProduct(t *testing.T) {
	e := newTestEnv(t)
	admin := e.adminToken()
	user := e.register("user@shop.test").AccessToken
	p := e.createProduct(admin, map[string]any{"name": "Gone soon", "price": 12.5, "stock": 3})

	e.addToCart(user, p.ID, 2)
	order := e.checkout(user)
	e.expect(http.StatusNoContent, "DELETE", "/api/products/"+itoa(p.ID), admin, nil)

	var got struct{ Order Order }
	e.expect(http.StatusOK, "GET", "/api/orders/"+itoa(order.ID), user, nil).decode(t, &got)
	item := got.Order.Items[0]
	if item.ProductID != nil || item.Name != "Gone soon" || item.Price != 12.5 || item.Subtotal != 25 {
		t.Fatalf("unexpected order item %+v", item)
	}
	// Cancelling must not fail when the product no longer exists.
	e.expect(http.StatusOK, "POST", "/api/orders/"+itoa(order.ID)+"/cancel", user, nil)
}

func TestConcurrentCheckoutCreatesOneOrder(t *testing.T) {
	e := newTestEnv(t)
	user := e.register("user@shop.test").AccessToken
	e.addToCart(user, 1, 1)

	counts := parallel(t, 10, func() (response, error) {
		return e.send("POST", "/api/orders", user, map[string]any{"shippingAddress": "Tashkent"})
	})
	if counts[http.StatusCreated] != 1 || counts[http.StatusBadRequest] != 9 {
		t.Fatalf("got status counts %v, want one 201 and nine 400", counts)
	}
	if stock := e.product(1).Stock; stock != 24 {
		t.Fatalf("stock = %d, want 24", stock)
	}
}

func TestConcurrentCancelRestocksOnce(t *testing.T) {
	e := newTestEnv(t)
	user := e.register("user@shop.test").AccessToken
	e.addToCart(user, 1, 3)
	order := e.checkout(user)

	counts := parallel(t, 10, func() (response, error) {
		return e.send("POST", "/api/orders/"+itoa(order.ID)+"/cancel", user, nil)
	})
	if counts[http.StatusOK] != 1 || counts[http.StatusBadRequest] != 9 {
		t.Fatalf("got status counts %v, want one 200 and nine 400", counts)
	}
	if stock := e.product(1).Stock; stock != 25 {
		t.Fatalf("stock = %d, want 25", stock)
	}
}
