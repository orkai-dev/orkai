#!/bin/sh
set -e

# Start API server and worker in background
orkai-api &
API_PID=$!
orkai-worker &
WORKER_PID=$!

# Wait for API to be ready
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Start Caddy (foreground)
caddy run --config /etc/caddy/Caddyfile &
CADDY_PID=$!

# Graceful shutdown
shutdown() {
    kill "$CADDY_PID" 2>/dev/null
    kill "$WORKER_PID" 2>/dev/null
    kill "$API_PID" 2>/dev/null
    wait "$API_PID" 2>/dev/null
    wait "$WORKER_PID" 2>/dev/null
    exit 0
}

trap shutdown SIGTERM SIGINT

# Wait for any of the three processes to exit. The worker MUST be watched too:
# without it, a worker crash leaves the container "healthy" (Caddy + API still
# answer /healthz) while deploy and backup jobs silently stop being processed.
wait -n "$API_PID" "$WORKER_PID" "$CADDY_PID" 2>/dev/null || true
shutdown
