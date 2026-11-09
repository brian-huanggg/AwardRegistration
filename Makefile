# Repo-level developer entry points. Backend targets live in backend/Makefile.
COMPOSE ?= docker compose

.PHONY: dev-up dev-down dev-reset migrate smoke

## Start MariaDB and MinIO in the background (waits for MariaDB to be healthy).
dev-up:
	$(COMPOSE) up -d --wait mariadb minio

## Stop the stack, keep data volumes.
dev-down:
	$(COMPOSE) down

## Stop the stack and delete all local data.
dev-reset:
	$(COMPOSE) down -v

## Apply golang-migrate migrations from backend/db/migration (idempotent). Uses ESG_DB_DSN, default = local compose DB.
migrate:
	cd backend && go run ./cmd/migrate

## Q3-01 smoke (S3 exit): compose up, migrate, MinIO bucket, open deadline, then Playwright starts backend + Vite.
smoke: dev-up migrate
	cd backend && go run ./cmd/mkbucket
	$(COMPOSE) exec -T mariadb mariadb -uesg -pesg esg -e "UPDATE contest_setting SET deadline = UTC_TIMESTAMP() + INTERVAL 1 DAY WHERE id = 1"
	cd frontend && npx playwright install chromium && npx playwright test
