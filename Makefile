include .env
export

MIGRATIONS_DIR=migrations
MIGRATE=go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: migrate-up migrate-down migrate-force

migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-force:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" force 1