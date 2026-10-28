package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"sort"
	"time"
)

type OrderItem struct {
	SKU string `json:"sku"`
	Name string `json:"name"`
	Qty int `json:"qty"`
	UnitPrice int `json:"unit_price"`
	Subtotal int `json:"subtotal"`
}

type Order struct {
	ID int `json:"id"`
	Customer string `json:"customer"`
	Status string `json:"status"`
	Total int `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	CancelledAt *time.Time `json:"cancelled_at"`
	Items []OrderItem `json:"items"`
}

type OrderItemInput struct {
	SKU string
	Qty int
}

func (s *Store) CreateOrder(ctx context.Context, customer string, items []OrderItemInput) (Order, error) {
	var order Order
	sorted := append([]OrderItemInput(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].SKU < sorted[j].SKU })

	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := checkSKUs(ctx, tx, sorted); err != nil {
			return err
		}

		var total int64
		lines := make([]OrderItem, 0, len(sorted))
		productIDs := make([]int, 0, len(sorted))
		for _, it := range sorted {
			var id int
			line := OrderItem{SKU: it.SKU, Qty: it.Qty}
			err := tx.QueryRow(ctx, `
				UPDATE products SET stock = stock - $2
				WHERE sku = $1 AND is_active AND stock >= $2
				RETURNING id, name, price`, it.SKU, it.Qty).Scan(&id, &line.Name, &line.UnitPrice)
			if errors.Is(err, pgx.ErrNoRows) {
				var avail int
				if err := tx.QueryRow(ctx, `SELECT stock FROM products WHERE sku = $1`, it.SKU).Scan(&avail); err != nil {
					return err
				}
				return &InsufficientStockError{SKU: it.SKU, Available: avail}
			}
			if err != nil {
				return err
			}
			total += int64(line.UnitPrice) * int64(it.Qty)
			if total > math.MaxInt32 {
				return &ValidationError{Details: []FieldError{{Field: "items", Message: "order total exceeds the maximum allowed"}}}
			}
			line.Subtotal = line.UnitPrice * it.Qty
			lines = append(lines, line)
			productIDs = append(productIDs, id)
		}

		order = Order{Customer: customer, Status: "CREATED", Total: int(total), Items: lines}
		err := tx.QueryRow(ctx,
			`INSERT INTO sales_orders (customer, total) VALUES ($1,$2) RETURNING id, created_at`,
			customer, order.Total).Scan(&order.ID, &order.CreatedAt)
		if err != nil {
			return err
		}
		note := fmt.Sprintf("Sales order #%d", order.ID)
		for i, l := range lines {
			if _, err := tx.Exec(ctx,
				`INSERT INTO sales_order_items (order_id, sku, qty, unit_price) VALUES ($1,$2,$3,$4)`,
				order.ID, l.SKU, l.Qty, l.UnitPrice); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1,'out',$2,$3)`,
				productIDs[i], l.Qty, note); err != nil {
				return err
			}
		}
		return nil
	})
	return order, err
}

func checkSKUs(ctx context.Context, q queryer, items []OrderItemInput) error {
	skus := make([]string, len(items))
	for i, it := range items {
		skus[i] = it.SKU
	}
	rows, err := q.Query(ctx, `SELECT sku, is_active FROM products WHERE sku = ANY($1)`, skus)
	if err != nil {
		return err
	}
	defer rows.Close()
	active := map[string]bool{}
	for rows.Next() {
		var sku string
		var a bool
		if err := rows.Scan(&sku, &a); err != nil {
			return err
		}
		active[sku] = a
	}
	if err := rows.Err(); err != nil {
		return err
	}
	var details []FieldError
	for _, it := range items {
		a, ok := active[it.SKU]
		switch {
		case !ok:
			details = append(details, FieldError{Field: "items", Message: fmt.Sprintf("unknown sku %q", it.SKU)})
		case !a:
			details = append(details, FieldError{Field: "items", Message: fmt.Sprintf("product %q is inactive and cannot be ordered", it.SKU)})
		}
	}
	if len(details) > 0 {
		return &ValidationError{Details: details}
	}
	return nil
}

const orderSelect = `SELECT id, customer, status, total, created_at, cancelled_at FROM sales_orders`

func scanOrder(row pgx.Row) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.Customer, &o.Status, &o.Total, &o.CreatedAt, &o.CancelledAt)
	return o, err
}

func loadItems(ctx context.Context, q queryer, orderID int) ([]OrderItem, error) {
	rows, err := q.Query(ctx, `
		SELECT i.sku, p.name, i.qty, i.unit_price
		FROM sales_order_items i JOIN products p ON p.sku = i.sku
		WHERE i.order_id = $1 ORDER BY i.id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.SKU, &it.Name, &it.Qty, &it.UnitPrice); err != nil {
			return nil, err
		}
		it.Subtotal = it.Qty * it.UnitPrice
		items = append(items, it)
	}
	return items, rows.Err()
}

func getOrder(ctx context.Context, q queryer, id int) (Order, error) {
	o, err := scanOrder(q.QueryRow(ctx, orderSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, err
	}
	o.Items, err = loadItems(ctx, q, id)
	return o, err
}

func (s *Store) GetOrder(ctx context.Context, id int) (Order, error) {
	return getOrder(ctx, s.pool, id)
}

func (s *Store) ListOrders(ctx context.Context, limit int) ([]Order, error) {
	rows, err := s.pool.Query(ctx, orderSelect+` ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		o.Items = []OrderItem{}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) CancelOrder(ctx context.Context, id int) (Order, error) {
	var out Order
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE sales_orders SET status='CANCELLED', cancelled_at=now() WHERE id=$1 AND status='CREATED'`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sales_orders WHERE id=$1)`, id).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
			return ErrAlreadyCancelled
		}

		rows, err := tx.Query(ctx, `SELECT sku, qty FROM sales_order_items WHERE order_id=$1 ORDER BY sku`, id)
		if err != nil {
			return err
		}
		var items []OrderItemInput
		for rows.Next() {
			var it OrderItemInput
			if err := rows.Scan(&it.SKU, &it.Qty); err != nil {
				rows.Close()
				return err
			}
			items = append(items, it)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		note := fmt.Sprintf("Cancelled sales order #%d", id)
		for _, it := range items {
			var pid int
			if err := tx.QueryRow(ctx, `UPDATE products SET stock = stock + $2 WHERE sku = $1 RETURNING id`,
				it.SKU, it.Qty).Scan(&pid); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO stock_movements (product_id, type, qty, note) VALUES ($1,'in',$2,$3)`,
				pid, it.Qty, note); err != nil {
				return err
			}
		}
		out, err = getOrder(ctx, tx, id)
		return err
	})
	return out, err
}
