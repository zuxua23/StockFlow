package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"stockflow/backend/internal/store"
	"strconv"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := map[string]any{"code": code, "message": message}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, status, map[string]any{"error": body})
}

func validationFailed(w http.ResponseWriter, details []store.FieldError) {
	msg := "request validation failed"
	if len(details) > 0 {
		msg = details[0].Message
	}
	writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", msg, map[string]any{"details": details})
}

func handleStoreError(w http.ResponseWriter, err error) {
	var (
		insufficient *store.InsufficientStockError
		validation *store.ValidationError
	)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found", nil)
	case errors.Is(err, store.ErrDuplicateSKU):
		writeError(w, http.StatusConflict, "SKU_ALREADY_EXISTS", "a product with this sku already exists", nil)
	case errors.Is(err, store.ErrHasHistory):
		writeError(w, http.StatusConflict, "PRODUCT_HAS_HISTORY", "product has movement history and cannot be deleted", nil)
	case errors.Is(err, store.ErrAlreadyCancelled):
		writeError(w, http.StatusConflict, "ORDER_ALREADY_CANCELLED", "order is already cancelled", nil)
	case errors.As(err, &insufficient):
		extra := map[string]any{"available": insufficient.Available}
		if insufficient.SKU != "" {
			extra["sku"] = insufficient.SKU
		}
		writeError(w, http.StatusConflict, "INSUFFICIENT_STOCK", insufficient.Error(), extra)
	case errors.As(err, &validation):
		validationFailed(w, validation.Details)
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error", nil)
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil {
		if _, err2 := dec.Token(); err2 != io.EOF {
			err = errors.New("request body must contain a single JSON object")
		}
	}
	if err == nil {
		return true
	}

	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &typeErr):
		validationFailed(w, []store.FieldError{{
			Field:   typeErr.Field,
			Message: fmt.Sprintf("%s must be of type %s", typeErr.Field, jsonTypeName(typeErr.Type.String())),
		}})
	case errors.Is(err, io.EOF):
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is empty", nil)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "malformed request body: "+err.Error(), nil)
	}
	return false
}

func jsonTypeName(goType string) string {
	switch goType {
	case "int", "int64", "int32", "*int":
		return "integer"
	case "string", "*string":
		return "string"
	case "bool", "*bool":
		return "boolean"
	}
	return goType
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found", nil)
		return 0, false
	}
	return id, true
}
