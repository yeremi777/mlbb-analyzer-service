#!/bin/sh
# Container entrypoint: apply migrations, seed dataset, then serve the API.
# Built to be idempotent, so a container restart never corrupts or duplicates.
set -euo pipefail

cd /app

# Database configuration is read from env (.env via env_file + explicit env in
# docker-compose.app.yml), same variables the Go app's config.DatabaseURL()
# uses. A URL is assembled here only for goose (a separate binary); the api and
# seed read DB_* themselves. When DATABASE_URL is provided it wins (that is
# the eventual direction in TASKS.md).
DB_USER="${DB_USERNAME:-postgres}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-mlbb_analyzer}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

if [ -n "${DATABASE_URL:-}" ]; then
    DB_URL="$DATABASE_URL"
else
    DB_USERINFO="${DB_USER}"
    if [ -n "${DB_PASSWORD:-}" ]; then
        DB_USERINFO="${DB_USER}:${DB_PASSWORD}"
    fi
    DB_URL="postgres://${DB_USERINFO}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"
fi

echo "=== migrations ==="
./goose -dir migrations postgres "$DB_URL" up

echo "=== seed ==="
./seed -data data/static

echo "=== api ==="
exec ./api