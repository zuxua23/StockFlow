package store

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound = errors.New("not found")
	ErrHasHistory = errors.New("product has history")
	ErrDuplicateSKU = errors.New("sku already exists")
	ErrAlreadyCancelled = errors.New("order already cancelled")
)

type InsufficientStockError struct {
	SKU string
	Available int
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for %s: %d available", e.SKU, e.Available)
}

type FieldError struct {
	Field string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Details []FieldError
}

func (e *ValidationError) Error() string {
	if len(e.Details) == 0 {
		return "validation failed"
	}
	return e.Details[0].Message
}
