package api

import (
	"fmt"
	"net/http"
	"stockflow/backend/internal/store"
	"strings"
)

const maxItemQty = 1_000_000

type orderBody struct {
	Customer *string `json:"customer"`
	Items []struct {
		SKU *string `json:"sku"`
		Qty *int `json:"qty"`
	} `json:"items"`
}

func (a *API) createOrder(w http.ResponseWriter, r *http.Request) {
	var body orderBody
	if !decodeBody(w, r, &body) {
		return
	}
	var errs []store.FieldError
	customer := ""
	if body.Customer == nil || strings.TrimSpace(*body.Customer) == "" {
		errs = append(errs, store.FieldError{Field: "customer", Message: "customer is required"})
	} else {
		customer = strings.TrimSpace(*body.Customer)
	}
	var items []store.OrderItemInput
	if len(body.Items) == 0 {
		errs = append(errs, store.FieldError{Field: "items", Message: "items must contain at least one item"})
	}
	seen := map[string]bool{}
	for i, it := range body.Items {
		field := fmt.Sprintf("items[%d]", i)
		sku := ""
		if it.SKU == nil || strings.TrimSpace(*it.SKU) == "" {
			errs = append(errs, store.FieldError{Field: field + ".sku", Message: fmt.Sprintf("%s: sku is required", field)})
		} else {
			sku = strings.TrimSpace(*it.SKU)
			if seen[sku] {
				errs = append(errs, store.FieldError{Field: field + ".sku", Message: fmt.Sprintf("%s: duplicate sku %q (combine quantities into one line)", field, sku)})
			}
			seen[sku] = true
		}
		switch {
		case it.Qty == nil || *it.Qty <= 0:
			errs = append(errs, store.FieldError{Field: field + ".qty", Message: fmt.Sprintf("%s: qty must be an integer > 0", field)})
		case *it.Qty > maxItemQty:
			errs = append(errs, store.FieldError{Field: field + ".qty", Message: fmt.Sprintf("%s: qty must be <= %d", field, maxItemQty)})
		case sku != "":
			items = append(items, store.OrderItemInput{SKU: sku, Qty: *it.Qty})
		}
	}
	if len(errs) > 0 {
		validationFailed(w, errs)
		return
	}

	order, err := a.store.CreateOrder(r.Context(), customer, items)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	o, err := a.store.GetOrder(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (a *API) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	o, err := a.store.CancelOrder(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (a *API) listOrders(w http.ResponseWriter, r *http.Request) {
	os, err := a.store.ListOrders(r.Context(), 50)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": os})
}
