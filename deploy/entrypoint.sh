#!/bin/sh
# Starts both processes and keeps the container alive only while both are up: a crashed API
# behind a healthy nginx would otherwise serve the UI and fail every calculation.
set -eu

# SIGQUIT is included because the nginx base image sets STOPSIGNAL SIGQUIT; PID 1 gets no
# default handler, so an untrapped stop signal is dropped and the container waits out the
# kill timeout instead of shutting down. Each process gets the signal it shuts down
# gracefully on: TERM for the API, QUIT for nginx, whose TERM is a fast exit.
terminate() {
    kill -TERM "$api" 2>/dev/null || true
    kill -QUIT "$web" 2>/dev/null || true
}
trap terminate TERM INT QUIT

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

api_status=0
web_status=0
wait "$api" || api_status=$?
wait "$web" || web_status=$?

# A process that died on its own carries its status out of the container, so a restart policy
# sees a failure. Both report 0 on a signalled shutdown, which is the normal path.
if [ "$api_status" -ne 0 ]; then
    exit "$api_status"
fi
exit "$web_status"
