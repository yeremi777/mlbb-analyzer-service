-include .env

DB_URL := postgres://$(DB_USERNAME):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
GOOSE  := go tool goose -dir migrations postgres "$(DB_URL)"

.PHONY: migrate-up migrate-down migrate-status seed test api build docs

migrate-up:
	$(GOOSE) up

migrate-down:
	$(GOOSE) down

migrate-status:
	$(GOOSE) status

seed:
	go run ./cmd/seed

test:
	go test ./...

run:
	go run ./cmd/api

build:
	go build ./...

docs:
	go tool swag init -g main.go -d ./cmd/api,./internal/api,./internal/staticdata -o internal/docs --parseInternal
