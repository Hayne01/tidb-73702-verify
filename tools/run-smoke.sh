#!/bin/bash
# Run the #73702 real-server smoke reproducer.
# Usage: run-smoke.sh <server-binary> <port> <out-prefix> <src-dir>
# Starts a fresh unistore tidb-server, runs the UNION smoke client on fresh
# connections, then reports whether the server process survived and whether it
# logged a concurrent-map fatal.
set -uo pipefail
SERVER="$1"; PORT="$2"; OUT="$3"; SRC="$4"

"$SERVER" --store=unistore --path="" --host=127.0.0.1 -P "$PORT" > "${OUT}.server.log" 2>&1 &
PID=$!
echo "server pid=$PID port=$PORT"

ready=0
for i in $(seq 1 90); do
  if grep -q "server is running MySQL protocol" "${OUT}.server.log" 2>/dev/null; then ready=1; break; fi
  if ! kill -0 "$PID" 2>/dev/null; then echo "server exited during startup"; break; fi
  sleep 1
done
echo "ready=$ready"
sleep 2

# Run the smoke client from inside the tidb module (uses its mysql driver dep).
mkdir -p "$SRC/cmd/smoke73702"
cp "${TOOLS_DIR}/smoke.go" "$SRC/cmd/smoke73702/main.go"
( cd "$SRC" && go run ./cmd/smoke73702 "$PORT" ) > "${OUT}.client.log" 2>&1 || true
cat "${OUT}.client.log"
sleep 2

if kill -0 "$PID" 2>/dev/null; then
  echo "SERVER_ALIVE"
else
  echo "SERVER_DEAD"
fi
if grep -qE "fatal error: concurrent map (read and map write|writes)" "${OUT}.server.log"; then
  echo "CONCURRENT_MAP_FATAL"
  grep -E "fatal error: concurrent map" "${OUT}.server.log" | head -1
fi
# Show the goroutine creator frames if present (UnionExec / projection).
grep -E "resultPuller|projectionInputFetcher|GetSessionOrGlobalSystemVar|setDataForVariablesInfo" "${OUT}.server.log" | head -6 || true

kill "$PID" 2>/dev/null || true
wait "$PID" 2>/dev/null || true
rm -rf "$SRC/cmd/smoke73702"