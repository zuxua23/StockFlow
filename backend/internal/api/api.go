package api

import (
	"log"
	"net/http"
	"stockflow/backend/internal/store"
	"time"
)

type API struct {
	store *store.Store
}

func New(s *store.Store) http.Handler {
	a := &API{store: s}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/products", a.listProducts)
	mux.HandleFunc("GET /api/products/summary", a.productSummary)
	mux.HandleFunc("POST /api/products", a.createProduct)
	mux.HandleFunc("PUT /api/products/{id}", a.updateProduct)
	mux.HandleFunc("DELETE /api/products/{id}", a.deleteProduct)
	mux.HandleFunc("POST /api/products/{id}/stock", a.adjustStock)
	mux.HandleFunc("GET /api/products/{id}/movements", a.listMovements)

	mux.HandleFunc("GET /api/orders", a.listOrders)
	mux.HandleFunc("POST /api/orders", a.createOrder)
	mux.HandleFunc("GET /api/orders/{id}", a.getOrder)
	mux.HandleFunc("POST /api/orders/{id}/cancel", a.cancelOrder)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found", nil)
	})

	return recoverer(logging(mux))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v", v)
				writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
