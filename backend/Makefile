# Online Shopping backend — local development and systemd deployment.
# Paths are always quoted because the project lives in a directory with spaces.

SERVICE   ?= online-shopping
APP_DIR   := $(CURDIR)
GO        ?= go
BIN       := bin/shop-api
RUN_USER  ?= $(shell id -un)
RUN_GROUP ?= $(shell id -gn)

# Database settings come from .env (created on first use); these are fallbacks.
-include .env
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= shop
DB_NAME ?= shop

UNIT_TEMPLATE := deploy/$(SERVICE).service.in
UNIT_BUILD    := build/$(SERVICE).service
UNIT_TARGET   := /etc/systemd/system/$(SERVICE).service

.DEFAULT_GOAL := help
.PHONY: help deps build env seed dev start vet test test-race db-create db-drop db-reset psql \
        service-unit service-install service-uninstall service-start service-stop \
        service-restart service-status service-logs clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' Makefile | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------- development

deps: ## Download Go modules
	$(GO) mod download

build: ## Build the server binary into bin/
	$(GO) build -trimpath -o "$(BIN)" ./cmd/shop-api

env: .env ## Create .env with random JWT_SECRET, DB_PASSWORD and ADMIN_PASSWORD (if missing)

# Only runs when .env does not exist; make then re-reads it via -include above.
.env:
	@rand() { od -An -tx1 -N"$$1" /dev/urandom | tr -d ' \n'; }; \
	sed -e "s|^JWT_SECRET=.*|JWT_SECRET=$$(rand 32)|" \
	    -e "s|^DB_PASSWORD=.*|DB_PASSWORD=$$(rand 16)|" \
	    -e "s|^ADMIN_PASSWORD=.*|ADMIN_PASSWORD=$$(rand 12)|" .env.example > .env
	@echo "Created .env with random secrets (the admin password is in ADMIN_PASSWORD)"

seed: .env build ## Create the admin user and sample products
	"./$(BIN)" seed

dev: .env ## Run the API from source
	$(GO) run ./cmd/shop-api

start: .env build ## Build and run the API in the foreground
	"./$(BIN)"

vet: ## Check formatting and run go vet
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; echo "run: gofmt -w cmd internal"; exit 1; }
	$(GO) vet ./...

test: ## Run all tests against a throwaway PostgreSQL (no sudo or running server needed)
	sh scripts/with-test-db.sh $(GO) test -count=1 ./...

test-race: ## Run all tests with the race detector (needs a C compiler)
	sh scripts/with-test-db.sh $(GO) test -race -count=1 ./...

# ----------------------------------------------------------------- PostgreSQL
# db-create / db-drop talk to a local server as the postgres superuser.
# Tables are created by the app itself (migrations run on startup).

db-create: .env ## Create the database role and database from .env (sudo)
	sudo -u postgres psql -p "$(DB_PORT)" -v db_user="$(DB_USER)" -v db_password="$(DB_PASSWORD)" \
		-v db_name="$(DB_NAME)" < deploy/create-db.sql

db-drop: ## Drop the database, deleting all data (sudo)
	@printf 'Drop database "%s" and all its data? [y/N] ' "$(DB_NAME)"; read ans; [ "$$ans" = y ]
	sudo -u postgres dropdb -p "$(DB_PORT)" --if-exists "$(DB_NAME)"

db-reset: ## Drop, recreate and re-seed the database (sudo)
	$(MAKE) db-drop
	$(MAKE) db-create
	$(MAKE) seed

psql: .env ## Open a psql shell on the app database
	PGPASSWORD="$(DB_PASSWORD)" psql -h "$(DB_HOST)" -p "$(DB_PORT)" -U "$(DB_USER)" "$(DB_NAME)"

# -------------------------------------------------------------------- systemd

service-unit: ## Render the systemd unit into build/
	@mkdir -p build
	sed -e 's|@APP_DIR@|$(APP_DIR)|g' \
	    -e 's|@BIN@|$(BIN)|g' \
	    -e 's|@USER@|$(RUN_USER)|g' \
	    -e 's|@GROUP@|$(RUN_GROUP)|g' \
	    "$(UNIT_TEMPLATE)" > "$(UNIT_BUILD)"
	@echo "Rendered $(UNIT_BUILD)"

service-install: .env build service-unit ## Build, install, enable and start the systemd service (sudo)
	sudo install -m 0644 "$(UNIT_BUILD)" "$(UNIT_TARGET)"
	sudo systemctl daemon-reload
	sudo systemctl enable --now "$(SERVICE)"
	@sudo systemctl --no-pager status "$(SERVICE)" || true

service-uninstall: ## Stop, disable and remove the systemd service (sudo)
	-sudo systemctl disable --now "$(SERVICE)"
	sudo rm -f "$(UNIT_TARGET)"
	sudo systemctl daemon-reload

service-start: ## Start the service
	sudo systemctl start "$(SERVICE)"

service-stop: ## Stop the service
	sudo systemctl stop "$(SERVICE)"

service-restart: build ## Rebuild and restart the service (after code changes)
	sudo systemctl restart "$(SERVICE)"

service-status: ## Show service status
	systemctl --no-pager status "$(SERVICE)"

service-logs: ## Follow service logs
	journalctl -u "$(SERVICE)" -f

clean: ## Remove build output
	rm -rf bin build
