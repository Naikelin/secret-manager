.PHONY: help dev build test clean migrate seed docker-up docker-down kind-up kind-down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Run development environment
	docker compose up

dev-bg: ## Run development environment in background
	docker compose up -d

build: ## Build all services
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && npm run build

test: ## Run tests
	cd backend && go test ./...
	cd frontend && npm test

clean: ## Clean build artifacts
	rm -rf backend/bin
	rm -rf frontend/.next
	docker compose down -v

docker-up: ## Start Docker services
	docker compose up -d

docker-down: ## Stop Docker services
	docker compose down

docker-logs: ## View Docker logs
	docker compose logs -f

kind-up: ## Create Kind cluster and install FluxCD
	./scripts/setup-kind.sh
	./scripts/setup-flux.sh
	./scripts/setup-age.sh

kind-down: ## Delete Kind cluster
	kind delete cluster --name secretmanager

migrate: ## Run database migrations
	@echo "Migrations will be handled by GORM AutoMigrate in Phase 2"

seed: ## Seed database with test data
	@echo "Seed data will be created in Phase 2"

backend-dev: ## Run backend in development mode (with Air)
	cd backend && air

frontend-dev: ## Run frontend in development mode
	cd frontend && npm run dev

.PHONY: setup-sops
setup-sops: ## Setup SOPS with Age keys for local dev
	@./scripts/setup-sops-dev.sh

.PHONY: test-sops
test-sops: ## Test SOPS encryption/decryption
	@cd backend && go test ./internal/sops/... -v
