package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

type Product struct {
	ID int `json:"id"`
	SKU string `json:"sku"`
	Name string `json:"name"`
	Price int `json:"price"`
	Stock int `json:"stock"`
	IsActive bool `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	LastMovementAt *time.Time `json:"last_movement_at"`
}

type Movement struct {
	ID int `json:"id"`
	ProductID int `json:"product_id"`
	Type string `json:"type"`
	Qty int `json:"qty"`
	Note *string `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type ProductInput struct {
	SKU string
	Name string
	Price int
	Stock int
	IsActive *bool
}

type ProductPage struct {
	Data []Product `json:"data"`
	Page int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

const productSelect = `SELECT p.id, p.sku, p.name, p.price, p.stock, p.is_active, p.created_at,
       (SELECT max(m.created_at) FROM stock_movements m WHERE m.product_id = p.id) FROM products p`

func scanProduct(row pgx.Row) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Price, &p.Stock, &p.IsActive, &p.CreatedAt, &p.LastMovementAt)
	return p, err
}

func getProduct(ctx context.Context, q queryer, id int) (Product, error) {
	p, err := scanProduct(q.QueryRow(ctx, productSelect+` WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) GetProduct(ctx context.Context, id int) (Product, error) {
	return getProduct(ctx, s.pool, id)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

const LowStockThreshold = 10

type Summary struct {
	TotalProducts int `json:"total_products"`
	ActiveProducts int `json:"active_products"`
	TotalStockValue int64 `json:"total_stock_value"`
	LowStockCount int `json:"low_stock_count"`
}

func (s *Store) ProductSummary(ctx context.Context) (Summary, error) {
	var out Summary
	err := s.pool.QueryRow(ctx, `
SELECT count(*),
       count(*) FILTER (WHERE is_active),
       COALESCE(sum(stock::bigint * price::bigint), 0),
       count(*) FILTER (WHERE stock <= $1)
FROM products`, LowStockThreshold).Scan(&out.TotalProducts, &out.ActiveProducts, &out.TotalStockValue, &out.LowStockCount)
	return out, err
}

func (s *Store) ListProducts(ctx context.Context, search, status string, page, limit int) (ProductPage, error) {
	out := ProductPage{Data: []Product{}, Page: page, Limit: limit}

	var conds []string
	var args []any
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		conds = append(conds, "(p.sku ILIKE $"+itoa(len(args))+" OR p.name ILIKE $"+itoa(len(args))+")")
	}
	switch status {
	case "active":
		conds = append(conds, "p.is_active")
	case "inactive":
		conds = append(conds, "NOT p.is_active")
	case "low":
		args = append(args, LowStockThreshold)
		conds = append(conds, "p.stock <= $"+itoa(len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM products p`+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}

	n := len(args)
	args = append(args, limit, (page-1)*limit)
	rows, err := s.pool.Query(ctx,
		productSelect+where+` ORDER BY p.id LIMIT $`+itoa(n+1)+` OFFSET $`+itoa(n+2), args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return out, err
		}
		out.Data = append(out.Data, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProduct(ctx context.Context, in ProductInput) (Product, error) {
	var out Product
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		active := true
		if in.IsActive != nil {
			active = *in.IsActive
		}
		var id int
		err := tx.QueryRow(ctx,
			`INSERT INTO products (sku, name, price, stock, is_active) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			in.SKU, in.Name, in.Price, in.Stock, active).Scan(&id)
		if err != nil {
			return err
		}
		if in.Stock > 0 {
			if _, err := tx.Exec(ctx,
				`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1,'in',$2,'Initial stock')`,
				id, in.Stock); err != nil {
				return err
			}
		}
		out, err = getProduct(ctx, tx, id)
		return err
	})
	if pgCode(err) == pgUniqueViolation {
		return out, ErrDuplicateSKU
	}
	return out, err
}

func (s *Store) UpdateProduct(ctx context.Context, id int, in ProductInput) (Product, error) {
	var out Product
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE products SET sku=$2, name=$3, price=$4, is_active=COALESCE($5, is_active) WHERE id=$1`,
			id, in.SKU, in.Name, in.Price, in.IsActive)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		out, err = getProduct(ctx, tx, id)
		return err
	})
	if pgCode(err) == pgUniqueViolation {
		return out, ErrDuplicateSKU
	}
	return out, err
}

func (s *Store) DeleteProduct(ctx context.Context, id int) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM products p WHERE p.id = $1
  		AND NOT EXISTS (SELECT 1 FROM stock_movements m WHERE m.product_id = p.id)
  		AND NOT EXISTS (SELECT 1 FROM sales_order_items i WHERE i.sku = p.sku)`, id)
	if pgCode(err) == pgFKViolation {
		return ErrHasHistory
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrHasHistory
}

type AdjustResult struct {
	Product Product `json:"product"`
	Movement Movement `json:"movement"`
}

func (s *Store) AdjustStock(ctx context.Context, id int, typ string, qty int, note *string) (AdjustResult, error) {
	var res AdjustResult
	delta := qty
	if typ == "out" {
		delta = -qty
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		var stock int
		err := tx.QueryRow(ctx,
			`UPDATE products SET stock = stock + $2 WHERE id = $1 AND stock + $2 >= 0 RETURNING stock`,
			id, delta).Scan(&stock)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, id).Scan(&stock)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			return &InsufficientStockError{Available: stock}
		}
		if err != nil {
			return err
		}
		m := &res.Movement
		err = tx.QueryRow(ctx,
			`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1,$2,$3,$4) RETURNING id, product_id, type, qty, note, created_at`,
			id, typ, qty, note).Scan(&m.ID, &m.ProductID, &m.Type, &m.Qty, &m.Note, &m.CreatedAt)
		if err != nil {
			return err
		}
		res.Product, err = getProduct(ctx, tx, id)
		return err
	})
	if pgCode(err) == pgNumericOutOfRng {
		return res, &ValidationError{Details: []FieldError{{Field: "qty", Message: "resulting stock would exceed the maximum allowed"}}}
	}
	return res, err
}

func (s *Store) ListMovements(ctx context.Context, productID int) ([]Movement, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`, productID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, product_id, type, qty, note, created_at FROM stock_movements
		 WHERE product_id = $1 ORDER BY created_at DESC, id DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Movement{}
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Type, &m.Qty, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
