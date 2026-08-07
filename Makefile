.PHONY: build test vet fmt up down logs ps migrate seed demo smoke dashboard run-%

GO ?= go

build: ## compile every service
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

up: ## start postgres + all services
	docker compose up --build -d

down:
	docker compose down

reset: ## start over, dropping the database volume
	docker compose down -v

logs:
	docker compose logs -f

ps:
	docker compose ps

# Migrations are applied by the one-shot `migrate` compose service on every
# `make up`; this target re-runs them against an already-running stack.
migrate:
	docker compose run --rm migrate

seed: ## believable demo users + orders (plan §10)
	./scripts/seed.sh

demo: ## curl walkthrough of the full happy path (plan §9)
	./scripts/demo.sh

smoke: ## /health on every service
	./scripts/smoke.sh

dashboard: ## admin dashboard in dev mode
	cd dashboard && npm run dev

# Run one service locally against dockerized postgres: make run-auth
run-%:
	$(GO) run ./cmd/$*
