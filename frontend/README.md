# Online Shopping — Frontend

React 19 + TypeScript single-page app, built with Vite into static files. The interface is in Uzbek,
Russian and English (switcher in the header) and prices are in so'm.

```
src/api/         API client: typed endpoints, token storage, automatic token refresh
src/auth/        logged-in user state (login, register, logout, session expiry)
src/cart/        cart state shared by the header and pages
src/i18n/        translations (uz, ru, en), price/date formatting, error messages
src/components/  layout, product card, shared UI pieces
src/pages/       catalog, product, cart + checkout, orders, login/register, admin
```

## Development

The dev server proxies `/api` to the backend on `127.0.0.1:5000`, so start the API first
(`make dev` in `backend/`), then:

```bash
make dev     # http://localhost:5173
make test    # unit tests
make check   # type-check
make build   # production build into dist/
```

Run `make` alone for all targets. Requires Node.js 20+.

## How it talks to the API

- Requests go to the same origin (`/api/...`). In production nginx serves `dist/` and proxies `/api/` to the
  Go server (see [`../deploy/nginx.conf`](../deploy/nginx.conf)), so no CORS setup is needed. To call an API
  on another domain instead, build with `VITE_API_URL=https://api.example.com make build` and add this site to
  the backend's `CORS_ORIGINS`.
- Access and refresh tokens are kept in `localStorage`, shared by all tabs. When a request gets `401`, the client
  refreshes the tokens and retries. The backend treats a reused refresh token as theft, so refreshes never
  overlap: parallel requests share one refresh, and tabs take turns through a Web Lock.
- The API's error messages are English; [`src/i18n/format.ts`](src/i18n/format.ts) maps the ones shoppers can
  see to translated text.

## Translations

All UI text is in [`src/i18n/messages.ts`](src/i18n/messages.ts). English defines the keys and TypeScript
fails the build if Uzbek or Russian is missing one; a unit test checks that `{placeholders}` match. Category
names for the shop's categories are translated there too. Product names and descriptions come from the
database as entered.
