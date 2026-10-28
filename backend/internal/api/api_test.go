package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stockflow/backend/internal/store"
)

func newServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := store.New(pool)
	return New(s), s
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Error == nil {
		t.Fatalf("not an error envelope: %q", rec.Body.String())
	}
	return out.Error
}

func newProduct(t *testing.T, s *store.Store, stock int) store.Product {
	t.Helper()
	sku := fmt.Sprintf("A-%d", time.Now().UnixNano())
	p, err := s.CreateProduct(context.Background(), store.ProductInput{SKU: sku, Name: sku, Price: 1000, Stock: stock})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInsufficientStockOrderReturns409AndWritesNothing(t *testing.T) {
	h, s := newServer(t)
	p := newProduct(t, s, 2)

	rec := do(h, "POST", "/api/orders", fmt.Sprintf(`{"customer":"x","items":[{"sku":%q,"qty":5}]}`, p.SKU))
	if rec.Code != 409 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	e := errorBody(t, rec)
	if e["code"] != "INSUFFICIENT_STOCK" || e["sku"] != p.SKU || e["available"] != float64(2) {
		t.Fatalf("error body = %v", e)
	}
	got, _ := s.GetProduct(context.Background(), p.ID)
	if got.Stock != 2 {
		t.Fatalf("stock = %d, want 2", got.Stock)
	}
	ms, _ := s.ListMovements(context.Background(), p.ID)
	if len(ms) != 1 {
		t.Fatalf("movements = %d, want 1 (initial only)", len(ms))
	}
}

func TestOrderValidation(t *testing.T) {
	h, s := newServer(t)
	p := newProduct(t, s, 5)
	cases := map[string]string{
		"empty items":   `{"customer":"x","items":[]}`,
		"qty zero":      fmt.Sprintf(`{"customer":"x","items":[{"sku":%q,"qty":0}]}`, p.SKU),
		"duplicate sku": fmt.Sprintf(`{"customer":"x","items":[{"sku":%q,"qty":1},{"sku":%q,"qty":1}]}`, p.SKU, p.SKU),
		"no customer":   fmt.Sprintf(`{"customer":"","items":[{"sku":%q,"qty":1}]}`, p.SKU),
		"unknown sku":   `{"customer":"x","items":[{"sku":"NOPE-404","qty":1}]}`,
		"client price":  fmt.Sprintf(`{"customer":"x","items":[{"sku":%q,"qty":1,"price":1}]}`, p.SKU),
	}
	for name, body := range cases {
		rec := do(h, "POST", "/api/orders", body)
		if rec.Code != 400 {
			t.Errorf("%s: status = %d, body %s", name, rec.Code, rec.Body)
		}
	}
	got, _ := s.GetProduct(context.Background(), p.ID)
	if got.Stock != 5 {
		t.Fatalf("stock = %d, want 5", got.Stock)
	}
}

func TestStockOverdrawReturns409WithAvailable(t *testing.T) {
	h, s := newServer(t)
	p := newProduct(t, s, 3)
	rec := do(h, "POST", fmt.Sprintf("/api/products/%d/stock", p.ID), `{"type":"out","qty":10}`)
	if rec.Code != 409 {
		t.Fatalf("status = %d", rec.Code)
	}
	e := errorBody(t, rec)
	if e["code"] != "INSUFFICIENT_STOCK" || e["available"] != float64(3) {
		t.Fatalf("error body = %v", e)
	}
}

func TestDeleteProductWithHistoryReturns409(t *testing.T) {
	h, s := newServer(t)
	p := newProduct(t, s, 3)
	rec := do(h, "DELETE", fmt.Sprintf("/api/products/%d", p.ID), "")
	if rec.Code != 409 || errorBody(t, rec)["code"] != "PRODUCT_HAS_HISTORY" {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
}

func TestMissingResourcesReturn404(t *testing.T) {
	h, _ := newServer(t)
	for _, c := range [][2]string{
		{"PUT", "/api/products/999999999"},
		{"GET", "/api/orders/999999999"},
		{"POST", "/api/orders/999999999/cancel"},
	} {
		body := ""
		if c[0] == "PUT" {
			body = `{"sku":"a","name":"b","price":1}`
		}
		if rec := do(h, c[0], c[1], body); rec.Code != 404 {
			t.Errorf("%s %s: status = %d", c[0], c[1], rec.Code)
		}
	}
}
