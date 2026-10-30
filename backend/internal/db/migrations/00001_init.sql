-- +goose Up
CREATE TABLE users (
  id            BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name          TEXT        NOT NULL,
  email         TEXT        NOT NULL UNIQUE CHECK (email = lower(email)),
  password_hash TEXT        NOT NULL,
  role          TEXT        NOT NULL DEFAULT 'customer' CHECK (role IN ('customer', 'admin')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Only a SHA-256 hash of each refresh token is stored. Rotated (revoked) rows are
-- kept until they expire so that reuse of an old token can be detected.
CREATE TABLE refresh_tokens (
  id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash TEXT        NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

CREATE TABLE products (
  id          BIGINT        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name        TEXT          NOT NULL,
  description TEXT          NOT NULL DEFAULT '',
  price       NUMERIC(12,2) NOT NULL CHECK (price >= 0),
  stock       INTEGER       NOT NULL DEFAULT 0 CHECK (stock >= 0),
  category    TEXT          NOT NULL DEFAULT 'general',
  image_url   TEXT,
  created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX products_category_idx ON products (category);

CREATE TABLE cart_items (
  user_id    BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  product_id BIGINT  NOT NULL REFERENCES products (id) ON DELETE CASCADE,
  quantity   INTEGER NOT NULL CHECK (quantity > 0),
  PRIMARY KEY (user_id, product_id)
);

CREATE TABLE orders (
  id               BIGINT        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id          BIGINT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  status           TEXT          NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'paid', 'shipped', 'delivered', 'cancelled')),
  total            NUMERIC(18,2) NOT NULL, -- room for many lines at the maximum price
  shipping_address TEXT          NOT NULL,
  created_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX orders_user_id_idx ON orders (user_id);

-- Name and price are copied so an order stays intact if the product changes or is deleted.
CREATE TABLE order_items (
  id         BIGINT        GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  order_id   BIGINT        NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
  product_id BIGINT        REFERENCES products (id) ON DELETE SET NULL,
  name       TEXT          NOT NULL,
  price      NUMERIC(12,2) NOT NULL,
  quantity   INTEGER       NOT NULL CHECK (quantity > 0)
);
CREATE INDEX order_items_order_id_idx ON order_items (order_id);

-- +goose Down
DROP TABLE order_items, orders, cart_items, products, refresh_tokens, users;
