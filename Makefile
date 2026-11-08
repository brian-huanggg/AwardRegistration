# Repo-level developer entry points. Backend targets live in backend/Makefile.
COMPOSE ?= docker compose

.PHONY: dev-up dev-down dev-reset migrate

## Start MariaDB and MinIO in the background (waits for MariaDB to be healthy).
dev-up:
	$(COMPOSE) up -d --wait mariadb minio

## Stop the stack, keep data volumes.
dev-down:
	$(COMPOSE) down

## Stop the stack and delete all local data.
dev-reset:
	$(COMPOSE) down -v

## Apply Flyway migrations from backend/db/migration (idempotent).
migrate:
	$(COMPOSE) run --rm flyway
