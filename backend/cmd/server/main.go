package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
	"os/signal"
	"stockflow/backend/internal/api"
	"stockflow/backend/internal/store"
	"syscall"
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	dsn := env("DATABASE_URL", "postgres://stockflow:stockflow@localhost:5432/stockflow?sslmode=disable")
	addr := ":" + env("PORT", "8080")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("db config: %v", err)
	}
	defer pool.Close()

	for i := 0; ; i++ {
		if err = pool.Ping(ctx); err == nil {
			break
		}
		if i >= 30 {
			log.Fatalf("db unreachable: %v", err)
		}
		log.Printf("waiting for db: %v", err)
		time.Sleep(time.Second)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(store.New(pool)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	log.Printf("listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
