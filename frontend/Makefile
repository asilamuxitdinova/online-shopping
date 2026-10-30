# Online Shopping frontend — React + TypeScript, built with Vite into dist/.

NPM ?= npm

.DEFAULT_GOAL := help
.PHONY: help deps dev build check test preview clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' Makefile | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

node_modules: package.json package-lock.json
	$(NPM) ci
	@touch node_modules

deps: node_modules ## Install npm dependencies exactly as locked

dev: node_modules ## Dev server on http://localhost:5173 (expects the API on 127.0.0.1:5000)
	$(NPM) run dev

build: node_modules ## Type-check and build static files into dist/
	$(NPM) run build

check: node_modules ## Type-check only
	$(NPM) run check

test: node_modules ## Run unit tests
	$(NPM) test

preview: build ## Serve the production build locally
	$(NPM) run preview

clean: ## Remove the build output
	rm -rf dist
