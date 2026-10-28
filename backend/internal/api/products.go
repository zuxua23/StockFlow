package api

import (
	"net/http"
	"stockflow/backend/internal/store"
	"strconv"
	"strings"
)

const (
	maxInt32 = 2147483647
	defaultPerPage = 20
	maxPerPage = 100
)

type productBody struct {
	SKU *string `json:"sku"`
	Name *string `json:"name"`
	Price *int `json:"price"`
	Stock *int `json:"stock"`
	IsActive *bool `json:"is_active"`
}

func (b productBody) validate(allowStock bool) (store.ProductInput, []store.FieldError) {
	var errs []store.FieldError
	in := store.ProductInput{IsActive: b.IsActive}
	if b.SKU == nil || strings.TrimSpace(*b.SKU) == "" {
		errs = append(errs, store.FieldError{Field: "sku", Message: "sku is required"})
	} else {
		in.SKU = strings.TrimSpace(*b.SKU)
	}
	if b.Name == nil || strings.TrimSpace(*b.Name) == "" {
		errs = append(errs, store.FieldError{Field: "name", Message: "name is required"})
	} else {
		in.Name = strings.TrimSpace(*b.Name)
	}
	switch {
	case b.Price == nil:
		errs = append(errs, store.FieldError{Field: "price", Message: "price is required"})
	case *b.Price < 0:
		errs = append(errs, store.FieldError{Field: "price", Message: "price must be >= 0"})
	case *b.Price > maxInt32:
		errs = append(errs, store.FieldError{Field: "price", Message: "price is too large"})
	default:
		in.Price = *b.Price
	}
	if b.Stock != nil {
		switch {
		case !allowStock:
			errs = append(errs, store.FieldError{Field: "stock", Message: "stock cannot be edited here; use POST /api/products/{id}/stock"})
		case *b.Stock < 0 || *b.Stock > maxInt32:
			errs = append(errs, store.FieldError{Field: "stock", Message: "stock must be a non-negative integer"})
		default:
			in.Stock = *b.Stock
		}
	}
	return in, errs
}

func (a *API) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := 1, defaultPerPage
	var errs []store.FieldError
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs = append(errs, store.FieldError{Field: "page", Message: "page must be an integer >= 1"})
		} else {
			page = n
		}
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPerPage {
			errs = append(errs, store.FieldError{Field: "limit", Message: "limit must be an integer between 1 and " + strconv.Itoa(maxPerPage)})
		} else {
			limit = n
		}
	}
	status := q.Get("status")
	switch status {
	case "", "all", "active", "inactive", "low":
	default:
		errs = append(errs, store.FieldError{Field: "status", Message: "status must be one of all, active, inactive, low"})
	}
	if len(errs) > 0 {
		validationFailed(w, errs)
		return
	}
	res, err := a.store.ListProducts(r.Context(), q.Get("search"), status, page, limit)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) createProduct(w http.ResponseWriter, r *http.Request) {
	var body productBody
	if !decodeBody(w, r, &body) {
		return
	}
	in, errs := body.validate(true)
	if len(errs) > 0 {
		validationFailed(w, errs)
		return
	}
	p, err := a.store.CreateProduct(r.Context(), in)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body productBody
	if !decodeBody(w, r, &body) {
		return
	}
	in, errs := body.validate(false)
	if len(errs) > 0 {
		validationFailed(w, errs)
		return
	}
	p, err := a.store.UpdateProduct(r.Context(), id, in)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteProduct(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type stockBody struct {
	Type *string `json:"type"`
	Qty *int `json:"qty"`
	Note *string `json:"note"`
}

func (a *API) adjustStock(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body stockBody
	if !decodeBody(w, r, &body) {
		return
	}
	var errs []store.FieldError
	if body.Type == nil || (*body.Type != "in" && *body.Type != "out") {
		errs = append(errs, store.FieldError{Field: "type", Message: `type must be "in" or "out"`})
	}
	if body.Qty == nil || *body.Qty <= 0 {
		errs = append(errs, store.FieldError{Field: "qty", Message: "qty must be an integer > 0"})
	} else if *body.Qty > maxInt32 {
		errs = append(errs, store.FieldError{Field: "qty", Message: "qty is too large"})
	}
	if len(errs) > 0 {
		validationFailed(w, errs)
		return
	}
	var note *string
	if body.Note != nil {
		if n := strings.TrimSpace(*body.Note); n != "" {
			note = &n
		}
	}

	res, err := a.store.AdjustStock(r.Context(), id, *body.Type, *body.Qty, note)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (a *API) listMovements(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ms, err := a.store.ListMovements(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ms})
}

func (a *API) productSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := a.store.ProductSummary(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}
