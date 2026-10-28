# StockFlow

Warehouse app: manage products, adjust stock, and record sales orders that consume stock.
Postgres + Go API (`net/http`, pgx) + React/TypeScript (Vite).

## Run

```bash
docker compose up --build
```

- App: http://localhost:3000
- API: http://localhost:8080
- Postgres: `localhost:5432` (user / password / db: `stockflow`)

Migrations and seed (`backend/migrations/`) run only on the first start of an empty volume.
To reset: `docker compose down -v && docker compose up --build`.

Dev mode (apps on the host, only Postgres in Docker):

```bash
docker compose up -d postgres
cd backend  && go run ./cmd/server
cd frontend && npm install && npm run dev   # http://localhost:5173
```

The dev frontend reaches the API through the Vite proxy (`/api` -> `localhost:8080`, in `frontend/vite.config.ts`),
so no CORS setup is needed. In Docker, nginx does the same.

## Tests

```bash
# backend (integration tests, need a database; skipped when TEST_DATABASE_URL is unset)
docker compose exec -T postgres psql -U stockflow -c "CREATE DATABASE stockflow_test"
docker compose exec -T postgres psql -U stockflow -d stockflow_test < backend/migrations/001_init.sql
cd backend
TEST_DATABASE_URL="postgres://stockflow:stockflow@localhost:5432/stockflow_test?sslmode=disable" go test -count=1 ./...

# with the race detector (needs cgo, so run it in a Linux container)
docker run --rm --network stockflow_default -v "$PWD/backend:/src" -w /src \
  -e TEST_DATABASE_URL="postgres://stockflow:stockflow@postgres:5432/stockflow_test?sslmode=disable" \
  golang:1.25 go test -race -count=1 ./...

# frontend (Vitest: validation and formatting)
cd frontend && npm test
```

Covered: insufficient stock is rejected and nothing is written; happy path writes the movement and updates stock;
cancel restores stock; 50 parallel orders on 10 units give exactly 10 successes and stock 0; parallel stock-outs never go negative;
parallel cancels restore stock once; opposite item orders do not deadlock; order validation.

## API

Errors use one shape: `{"error":{"code":"...","message":"...", ...}}`.
Codes: `VALIDATION_ERROR` (with `details`), `INVALID_JSON`, `NOT_FOUND`, `SKU_ALREADY_EXISTS`, `PRODUCT_HAS_HISTORY`,
`INSUFFICIENT_STOCK` (with `available`, and `sku` for orders), `ORDER_ALREADY_CANCELLED`, `INTERNAL`.

| Method | Path | Notes |
|---|---|---|
| GET | `/api/products?search=&status=&page=&limit=` | `{data,page,limit,total}`; `status` = `all\|active\|inactive\|low` |
| GET | `/api/products/summary` | totals for the summary cards |
| POST | `/api/products` | 201; optional `stock` is recorded as an `in` movement |
| PUT | `/api/products/{id}` | 404 if missing; does not change stock |
| DELETE | `/api/products/{id}` | 204, or 409 `PRODUCT_HAS_HISTORY` |
| POST | `/api/products/{id}/stock` | 201, or 409 `INSUFFICIENT_STOCK` |
| GET | `/api/products/{id}/movements` | `{data:[...]}`, newest first |
| GET | `/api/orders` | extra: latest 50 orders, used by the UI |
| POST | `/api/orders` | 201, server-computed total |
| GET | `/api/orders/{id}` | order with items |
| POST | `/api/orders/{id}/cancel` | restores stock; 409 if already cancelled |

On Windows cmd, put the JSON in a file and use `-d @file.json`. Replace SKUs with ones from your seed.

```bash
curl "localhost:8080/api/products?search=kardus&status=active"

curl -X POST localhost:8080/api/products/1/stock -H 'Content-Type: application/json' \
  -d '{"type":"out","qty":5,"note":"sample"}'

# 201
curl -X POST localhost:8080/api/orders -H 'Content-Type: application/json' \
  -d '{"customer":"Budi","items":[{"sku":"SKU-001","qty":2}]}'

# 409 INSUFFICIENT_STOCK with sku + available, nothing changes
curl -X POST localhost:8080/api/orders -H 'Content-Type: application/json' \
  -d '{"customer":"Budi","items":[{"sku":"SKU-001","qty":1},{"sku":"SKU-004","qty":999}]}'

# 400 with details
curl -X POST localhost:8080/api/orders -H 'Content-Type: application/json' \
  -d '{"customer":"","items":[{"sku":"X","qty":0},{"sku":"X","qty":1}]}'

curl -X POST localhost:8080/api/orders/1/cancel
curl -X DELETE localhost:8080/api/products/1
```

## Design decisions

- **No SELECT-then-UPDATE.** Stock changes are one guarded statement:
  `UPDATE products SET stock = stock - $qty WHERE sku = $1 AND is_active AND stock >= $qty RETURNING ...`.
  If no row matches, a follow-up read only decides which error to return. The `CHECK (stock >= 0)` is a safety net, not the mechanism.
- **An order is one transaction:** validate SKUs, decrement every item, insert order, items and movements. Any failure rolls back everything.
  Items are processed in SKU order so concurrent orders lock rows in the same order and cannot deadlock.
- **Prices and totals come from the server.** The JSON decoder rejects unknown fields, so a client-sent `price` is a 400. Money is integer IDR.
- **`last_movement_at` is derived** from `max(stock_movements.created_at)` (index on `product_id, created_at DESC`), so it cannot drift from the ledger.
- **The ledger is complete:** orders write `out` movements and cancels write `in` movements, so `stock = sum(in) - sum(out)`.
  Any product with history cannot be deleted (guard inside the `DELETE`, FK violation also mapped to 409).
- **Cancel** is a conditional `UPDATE ... WHERE status = 'CREATED'` that also acts as the lock, so concurrent cancels restore stock once.
- **Schema additions:** `ON UPDATE CASCADE` on `sales_order_items.sku`, plus lookup indexes.
- **Frontend:** `api.ts` owns every `fetch` and turns the error envelope into an `ApiError`; components branch on `code`.
  Filters and summary are server-side, so pagination and totals are right beyond one page.

## What I would improve

- Idempotency keys on `POST /api/orders`, so a retried request cannot create a duplicate.
- Optimistic locking on product edits (today the last `PUT` wins).
- A movements panel in the UI (endpoint and client method already exist), more filters and sorting.
- Component and e2e tests, a real migration tool, and CI running `go test -race` and lint.

## Time spent

7h 34m
