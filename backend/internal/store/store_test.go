package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestStore(t *testing.T) *Store {
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
	return New(pool)
}

func newProduct(t *testing.T, s *Store, stock, price int) Product {
	t.Helper()
	sku := fmt.Sprintf("T-%d", time.Now().UnixNano())
	p, err := s.CreateProduct(context.Background(), ProductInput{SKU: sku, Name: sku, Price: price, Stock: stock})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func stockOf(t *testing.T, s *Store, id int) int {
	t.Helper()
	p, err := s.GetProduct(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return p.Stock
}

func TestConcurrentOrdersNeverOversell(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := newProduct(t, s, 10, 1000)

	const workers = 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, insufficient, other := 0, 0, 0
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.CreateOrder(ctx, "c", []OrderItemInput{{SKU: p.SKU, Qty: 1}})
			var ins *InsufficientStockError
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.As(err, &ins):
				insufficient++
			default:
				other++
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok != 10 || insufficient != workers-10 || other != 0 {
		t.Fatalf("ok=%d insufficient=%d other=%d, want 10/%d/0", ok, insufficient, other, workers-10)
	}
	if got := stockOf(t, s, p.ID); got != 0 {
		t.Fatalf("final stock = %d, want 0", got)
	}
	ms, err := s.ListMovements(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	out := 0
	for _, m := range ms {
		if m.Type == "out" {
			out += m.Qty
		}
	}
	if out != 10 {
		t.Fatalf("out movements = %d, want 10", out)
	}
}

func TestCreateOrderHappyPath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := newProduct(t, s, 10, 2500)
	time.Sleep(2 * time.Millisecond)
	b := newProduct(t, s, 4, 40000)

	o, err := s.CreateOrder(ctx, "Budi", []OrderItemInput{{SKU: a.SKU, Qty: 3}, {SKU: b.SKU, Qty: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "CREATED" || o.Total != 3*2500+40000 || len(o.Items) != 2 {
		t.Fatalf("unexpected order: %+v", o)
	}
	if got := stockOf(t, s, a.ID); got != 7 {
		t.Fatalf("stock A = %d, want 7", got)
	}
	if got := stockOf(t, s, b.ID); got != 3 {
		t.Fatalf("stock B = %d, want 3", got)
	}
	ms, err := s.ListMovements(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Type != "out" || ms[0].Qty != 3 {
		t.Fatalf("movements of A = %+v, want newest to be out/3", ms)
	}
	got, err := s.GetOrder(ctx, o.ID)
	if err != nil || got.Total != o.Total || len(got.Items) != 2 {
		t.Fatalf("GetOrder = %+v, %v", got, err)
	}

	cancelled, err := s.CancelOrder(ctx, o.ID)
	if err != nil || cancelled.Status != "CANCELLED" {
		t.Fatalf("CancelOrder = %+v, %v", cancelled, err)
	}
	if got := stockOf(t, s, a.ID); got != 10 {
		t.Fatalf("stock A after cancel = %d, want 10", got)
	}
}

func TestDeleteProductWithHistoryIsRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	withHistory := newProduct(t, s, 5, 100)
	if err := s.DeleteProduct(ctx, withHistory.ID); !errors.Is(err, ErrHasHistory) {
		t.Fatalf("delete with history: err = %v, want ErrHasHistory", err)
	}
	time.Sleep(2 * time.Millisecond)
	clean := newProduct(t, s, 0, 100)
	if err := s.DeleteProduct(ctx, clean.ID); err != nil {
		t.Fatalf("delete without history: %v", err)
	}
	if err := s.DeleteProduct(ctx, clean.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: err = %v, want ErrNotFound", err)
	}
}

func TestFailedOrderChangesNothing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := newProduct(t, s, 5, 100)
	time.Sleep(2 * time.Millisecond)
	b := newProduct(t, s, 1, 100)

	_, err := s.CreateOrder(ctx, "c", []OrderItemInput{{SKU: a.SKU, Qty: 2}, {SKU: b.SKU, Qty: 5}})
	var ins *InsufficientStockError
	if !errors.As(err, &ins) || ins.SKU != b.SKU || ins.Available != 1 {
		t.Fatalf("err = %v, want insufficient for %s with 1 available", err, b.SKU)
	}
	if got := stockOf(t, s, a.ID); got != 5 {
		t.Fatalf("stock of A = %d, want 5", got)
	}
	if got := stockOf(t, s, b.ID); got != 1 {
		t.Fatalf("stock of B = %d, want 1", got)
	}
	ms, _ := s.ListMovements(ctx, a.ID)
	if len(ms) != 1 {
		t.Fatalf("movements of A = %d, want 1 (initial stock only)", len(ms))
	}
}

func TestOppositeItemOrderDoesNotDeadlock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := newProduct(t, s, 100, 10)
	time.Sleep(2 * time.Millisecond)
	b := newProduct(t, s, 100, 10)

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		items := []OrderItemInput{{SKU: a.SKU, Qty: 1}, {SKU: b.SKU, Qty: 1}}
		if i%2 == 1 {
			items[0], items[1] = items[1], items[0]
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.CreateOrder(ctx, "c", items); err != nil {
				t.Errorf("create order: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := stockOf(t, s, a.ID); got != 60 {
		t.Fatalf("stock A = %d, want 60", got)
	}
	if got := stockOf(t, s, b.ID); got != 60 {
		t.Fatalf("stock B = %d, want 60", got)
	}
}

func TestConcurrentStockOutNeverNegative(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := newProduct(t, s, 10, 100)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.AdjustStock(ctx, p.ID, "out", 1, nil)
			var ins *InsufficientStockError
			switch {
			case err == nil:
				mu.Lock()
				ok++
				mu.Unlock()
			case !errors.As(err, &ins):
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 10 {
		t.Fatalf("successful outs = %d, want 10", ok)
	}
	if got := stockOf(t, s, p.ID); got != 0 {
		t.Fatalf("final stock = %d, want 0", got)
	}
}

func TestConcurrentCancelRestoresStockOnce(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := newProduct(t, s, 10, 100)
	o, err := s.CreateOrder(ctx, "c", []OrderItemInput{{SKU: p.SKU, Qty: 4}})
	if err != nil {
		t.Fatal(err)
	}
	if got := stockOf(t, s, p.ID); got != 6 {
		t.Fatalf("stock after order = %d, want 6", got)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, already := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CancelOrder(ctx, o.ID)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrAlreadyCancelled):
				already++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || already != 9 {
		t.Fatalf("ok=%d already=%d, want 1/9", ok, already)
	}
	if got := stockOf(t, s, p.ID); got != 10 {
		t.Fatalf("stock after cancels = %d, want 10", got)
	}
}
