# Online Shopping — Backend API

Go REST API (standard library `net/http`) backed by PostgreSQL via [`pgx`](https://github.com/jackc/pgx).
Auth uses short-lived JWT access tokens plus rotating refresh tokens. Requires Go 1.22+ and PostgreSQL 13+.

```
cmd/shop-api/            entry point: `shop-api` serves, `shop-api seed` seeds
internal/config/         settings from the environment / .env
internal/db/             connection pool, transactions
internal/db/migrations/  SQL schema migrations (goose), embedded in the binary
internal/api/            HTTP handlers: auth, products, cart, orders
internal/seed/           admin user + sample products
internal/testdb/         per-test throwaway databases
deploy/                  systemd unit template, database setup script
scripts/                 test helper that runs a throwaway PostgreSQL
```

## Setup

Run `make` alone to list all targets. With PostgreSQL installed and running on the machine:

```bash
make db-create   # creates .env (random JWT_SECRET, DB_PASSWORD, ADMIN_PASSWORD), then the DB role and database (sudo)
make seed        # builds, applies migrations, adds admin + sample products
make dev         # http://127.0.0.1:5000
```

Other database targets: `make psql` (shell on the app database), `make db-reset` (drop, recreate, re-seed).

For development without touching the system PostgreSQL, `make local-db` instead starts a private server as
your user (no sudo) on port 55433, creates the database and sets `DB_PORT` in `.env`; stop it with
`make local-db-stop`. Data is kept in `~/.local/share/online-shopping/postgres` between runs.

### Configuration

All settings are read from `.env` (see `.env.example`); real environment variables take precedence.

| Variable | Default | Purpose |
|---|---|---|
| `HOST`, `PORT` | `127.0.0.1`, `5000` | Listen address. Keep `127.0.0.1` behind nginx. |
| `JWT_SECRET` | — (required) | ≥ 32 random characters; the server refuses to start otherwise. |
| `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL` | `15m`, `30d` | Token lifetimes. |
| `CORS_ORIGINS` | none | Comma-separated frontend origins allowed to call the API from a browser (`*` = any). |
| `TRUST_PROXY` | `false` | Use `X-Real-IP` as the client IP. Only enable behind nginx, which sets it. |
| `AUTH_RATE_PER_MINUTE` | `10` | Login/register/refresh attempts per client IP per minute; excess gets `429`. |
| `DB_*` / `DATABASE_URL` | see below | PostgreSQL connection. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | — | Admin created by `make seed`; the password must be ≥ 12 characters and not an example value. |

### Database

Connection settings live in `.env` as `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` and `DB_SSLMODE`.
Alternatively set `DATABASE_URL` (e.g. `postgres://user:pass@host:5432/shop?sslmode=require`), which takes
precedence — useful for a managed database, where you create the role and database yourself instead of
`make db-create`.

Migrations in `internal/db/migrations/` run automatically every time the binary starts; applied versions are
tracked in the `goose_db_version` table. To change the schema, add a new file such as
`00002_add_reviews.sql` with `-- +goose Up` / `-- +goose Down` sections — never edit one that has
already been applied.

## Tests

```bash
make test        # all tests against a throwaway PostgreSQL
make test-race   # same, with the race detector
```

`make test` starts a temporary PostgreSQL server in a temp directory (it needs the PostgreSQL server binaries,
not a running server or sudo; run it as a normal user, since `initdb` refuses to run as root) and gives
every test its own fresh, migrated database. To run against an existing server instead, set
`TEST_DATABASE_URL` to a superuser connection and run `go test ./...`; without it, database tests are skipped.

## Deploying to a server

See the [top-level README](../README.md#deploying-to-a-server) for the full setup with the frontend and nginx.
For the API alone:

```bash
make db-create          # creates .env with random secrets, then the database (sudo)
# edit .env: TRUST_PROXY=true (behind nginx)
make seed               # creates the admin (password is ADMIN_PASSWORD in .env)
make service-install    # builds and starts the systemd service on 127.0.0.1:5000
```

`CORS_ORIGINS` is only needed if the frontend is served from a different domain than the API.

## Running as a systemd service

```bash
make service-install   # builds bin/shop-api, installs the unit to /etc/systemd/system, enables + starts it
make service-status
make service-logs
make service-restart   # rebuilds and restarts after code changes
make service-uninstall
```

The unit runs `bin/shop-api` from this directory as the current user (override with
`make service-install RUN_USER=shop RUN_GROUP=shop`). Settings come from `.env`.
Re-run `make service-install` if you move the project. The unit starts after `postgresql.service`; if the
database is unreachable at startup the service exits and systemd retries every 5 seconds.

## Authentication

Register, log in and refresh all return a token pair:

```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "kq3V0u8...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": { "id": 2, "name": "...", "email": "...", "role": "customer", "created_at": "..." }
}
```

- **Access token** — short-lived JWT (`ACCESS_TOKEN_TTL`, default 15m). Send it on protected routes as
  `Authorization: Bearer <access_token>`.
- **Refresh token** — long-lived random string (`REFRESH_TOKEN_TTL`, default 30d), stored server-side as a
  SHA-256 hash. When the API answers `401` (or just before `expires_in` runs out), call `/api/auth/refresh`
  to get a new pair.
- **Rotation** — every refresh revokes the refresh token it was given; always keep the newest one. If a
  revoked refresh token is used again (a sign it was stolen), all of that user's sessions are revoked.
- Logging out revokes refresh tokens only; an access token stays valid until it expires.

| Method | Endpoint               | Body                        | Notes                    |
|--------|------------------------|-----------------------------|--------------------------|
| POST   | `/api/auth/register`   | `{ name, email, password }` | password ≥ 8 chars → token pair |
| POST   | `/api/auth/login`      | `{ email, password }`       | → token pair             |
| POST   | `/api/auth/refresh`    | `{ refreshToken }`          | → new token pair; old refresh token is revoked |
| POST   | `/api/auth/logout`     | `{ refreshToken }`          | revokes this session → `204` |
| POST   | `/api/auth/logout-all` | —                           | 🔒 revokes every session of the user → `204` |
| GET    | `/api/auth/me`         | —                           | 🔒 current user          |

## Products

| Method | Endpoint                   | Notes |
|--------|----------------------------|-------|
| GET    | `/api/products`            | Query: `search`, `category`, `minPrice`, `maxPrice`, `sort` (`newest`, `price_asc`, `price_desc`, `name`), `page`, `limit` |
| GET    | `/api/products/categories` | Categories with product counts |
| GET    | `/api/products/:id`        | |
| POST   | `/api/products`            | 🔒 admin — `{ name, price, description?, stock?, category?, image_url? }` |
| PATCH  | `/api/products/:id`        | 🔒 admin — any of the fields above |
| DELETE | `/api/products/:id`        | 🔒 admin |

## Cart (🔒 logged in)

| Method | Endpoint                       | Body                        |
|--------|--------------------------------|-----------------------------|
| GET    | `/api/cart`                    | —                           |
| POST   | `/api/cart/items`              | `{ productId, quantity? }` — adds to existing quantity |
| PATCH  | `/api/cart/items/:productId`   | `{ quantity }` — sets quantity |
| DELETE | `/api/cart/items/:productId`   | —                           |
| DELETE | `/api/cart`                    | — clears the cart           |

## Orders (🔒 logged in)

| Method | Endpoint                  | Notes |
|--------|---------------------------|-------|
| POST   | `/api/orders`             | `{ shippingAddress }` — checks out the cart, reduces stock |
| GET    | `/api/orders`             | Your orders (admin: `?all=true` for everyone's) |
| GET    | `/api/orders/:id`         | |
| POST   | `/api/orders/:id/cancel`  | Pending orders only; stock is returned |
| PATCH  | `/api/orders/:id/status`  | 🔒 admin — `{ status }`: `pending`, `paid`, `shipped`, `delivered`, `cancelled` |

Errors are returned as `{ "error": "message" }` with an appropriate HTTP status.

### Limits

Requests outside these limits get `400`: names ≤ 200 characters, descriptions ≤ 5000, shipping addresses
≤ 500, prices 0 – 1,000,000,000 (rounded to 2 decimals), stock 0 – 1,000,000,000, at most 1000 of one product
in a cart, and `image_url` must be an `http(s)` link (send `""` to clear it). Request bodies are capped at 1 MB.
Search matches `%` and `_` literally. `GET /api/health` returns `503` when the database is unreachable.
