#!/bin/sh
# Collector scheduler running inside the container. Loops forever, running the
# collector at the configured time each day (default 07:00 Asia/Jakarta, the
# same schedule deploy/cron/crontab used for host cron).
#
# Portable across busybox (Alpine) and GNU date: computes the next run purely
# from the current local epoch, avoiding `date -d tomorrow` / `date -d now`.
set -euo pipefail

cd /app

COLLECT_TIME="${COLLECT_TIME:-07:00}"
export TZ="${TZ:-Asia/Jakarta}"

# A leading zero makes the shell read the field as an octal constant, so "08"
# and "09" are errors rather than hours. Strip it before any arithmetic.
HH=${COLLECT_TIME%%:*}
MM=${COLLECT_TIME#*:}
HH=${HH#0}
MM=${MM#0}

collect() {
    echo "=== $(date +%FT%T%z) collector start ==="
    status=0
    ./collector all || status=$?
    echo "=== $(date +%FT%T%z) collector exit $status ==="
    return $status
}

sleep_until() {
    # Seconds since local midnight, read from the local clock rather than
    # derived from the epoch. Epoch arithmetic is always UTC, so deriving the
    # day boundary that way fired the run at 14:00 WIB for a 07:00 setting --
    # off by exactly the timezone offset. TZ shifts how date prints, never how
    # the shell counts.
    now_h=$(date +%H)
    now_m=$(date +%M)
    now_s=$(date +%S)
    cur=$(( ${now_h#0} * 3600 + ${now_m#0} * 60 + ${now_s#0} ))
    tgt=$(( HH * 3600 + MM * 60 ))

    wait=$(( tgt - cur ))
    if [ "$wait" -le 0 ]; then
        wait=$(( wait + 86400 ))
    fi
    echo "next collector run in ${wait}s (${COLLECT_TIME} ${TZ})"
    sleep "$wait"
}

while true; do
    sleep_until
    # No flock needed here: one scheduled process, sequential by construction.
    collect || true
done