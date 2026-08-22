#!/usr/bin/env bash
# Cron entrypoint for the collector.
#
# Cron runs with a minimal environment and $HOME as the working directory, so
# the app directory is absolute and the cd is mandatory: godotenv reads .env
# relative to the process working directory.
set -euo pipefail

APP_DIR="${APP_DIR:-/var/opt/mlbb-analyzer}"
LOG_DIR="${LOG_DIR:-/var/log/mlbb-analyzer}"

cd "$APP_DIR"
mkdir -p "$LOG_DIR"
exec >>"$LOG_DIR/collector.log" 2>&1

echo "=== $(date -Is) collector start ==="
status=0
"$APP_DIR/bin/collector" "$@" || status=$?
echo "=== $(date -Is) collector exit $status ==="
exit $status
