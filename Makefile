-include .env

DB_URL := postgres://$(DB_USERNAME):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
GOOSE  := go tool goose -dir migrations postgres "$(DB_URL)"

.PHONY: reset-raw migrate-up migrate-down migrate-status seed test api build docs collect collect-patches collect-stats build-collector

migrate-up:
	$(GOOSE) up

migrate-down:
	$(GOOSE) down

migrate-status:
	$(GOOSE) status

seed:
	go run ./cmd/seed

collect:
	go run ./cmd/collector all

collect-patches:
	go run ./cmd/collector patches

collect-stats:
	go run ./cmd/collector stats

build-collector:
	go build -o bin/collector ./cmd/collector

# Destructive: empties the raw zone and restarts id sequences, for a clean
# development reset. Sequences are non-transactional, so rolled-back test
# inserts leave permanent gaps; this is how you start over, not a way to keep
# ids contiguous.
reset-raw:
	@echo "This deletes every row in raw.hero_rank_snapshots and raw.patch_snapshots."
	@read -p "Type 'yes' to continue: " ok && [ "$$ok" = yes ]
	psql "$(DB_URL)" -c "TRUNCATE raw.hero_rank_snapshots, raw.patch_snapshots RESTART IDENTITY;"

test:
	go test ./...

run:
	go run ./cmd/api

build:
	go build ./...

docs:
	go tool swag init -g main.go -d ./cmd/api,./internal/api,./internal/domain -o internal/docs --parseInternal
