# Online Shopping

An online shop for Uzbekistan: a Go + PostgreSQL REST API and a React + TypeScript website in
Uzbek, Russian and English, with prices in so'm.

```
backend/    Go API: auth with access + refresh tokens, products, cart, orders, admin   → backend/README.md
frontend/   React SPA: catalog, cart and checkout, orders, admin panel                 → frontend/README.md
deploy/     nginx site serving both on one domain
```

## Running locally

Needs Go 1.22+, Node.js 20+ and the PostgreSQL server package (the system PostgreSQL service is not used).
In two terminals:

```bash
cd backend && make local-db && make seed && make dev      # API on 127.0.0.1:5000
```

```bash
cd frontend && make dev                                   # website on http://localhost:5173
```

`make local-db` runs a private PostgreSQL as your user on port 55433 (data in `~/.local/share/online-shopping`)
and points `backend/.env` at it; `make local-db-stop` stops it. The admin login is `ADMIN_EMAIL` /
`ADMIN_PASSWORD` from `backend/.env`. Tests: `make test` in each folder.

## Deploying to a server

On a Linux server with Go, Node.js, PostgreSQL and nginx (no Docker; the API runs as a systemd service and
the website is static files):

```bash
git clone git@github.com:asilamuxitdinova/online-shopping.git /opt/online-shopping

cd /opt/online-shopping/backend
make db-create          # creates .env with random secrets, then the database role and database (sudo)
# edit .env: TRUST_PROXY=true
make seed               # admin account + sample products
make service-install    # builds the API and starts it as a systemd service on 127.0.0.1:5000

cd ../frontend
make build              # static site in frontend/dist
```

Then install [`deploy/nginx.conf`](deploy/nginx.conf) (replace the domain and paths) and get a certificate
with `sudo certbot --nginx -d your-domain`.

To update later:

```bash
cd /opt/online-shopping && git pull
make -C backend service-restart
make -C frontend build
```
