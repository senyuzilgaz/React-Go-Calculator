#!/bin/sh
# Starts both processes and keeps the container alive only while both are up: a crashed API
# behind a healthy nginx would otherwise serve the UI and fail every calculation.
set -eu

terminate() {
    kill -TERM "$api" "$web" 2>/dev/null || true
}
trap terminate TERM INT

/usr/local/bin/server &
api=$!

nginx -g 'daemon off;' &
web=$!

# sh runs a trap only between foreground commands, so this polls instead of blocking on wait;
# the interval is the delay before a signal reaches the two children.
while kill -0 "$api" 2>/dev/null && kill -0 "$web" 2>/dev/null; do
    sleep 1
done

terminate
wait "$api" 2>/dev/null || true
wait "$web" 2>/dev/null || true
