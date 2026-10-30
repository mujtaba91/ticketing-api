.PHONY: up down migrate-up migrate-down test test-integration lint

.env:
	cp .env.example .env

up: .env
	docker compose up --build

down:
	docker compose down

migrate-up: .env
	docker compose run --rm migrate up

# Rolls back one migration at a time on purpose.
migrate-down: .env
	docker compose run --rm migrate down 1

test:
	go test ./...

test-integration:
	go test -tags=integration ./tests/integration/...

# Same version as CI, so no local install is needed.
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run
