#!/bin/bash

PORT=1112

echo "==> Checking existing mybox serve..."

PIDS=$(pgrep -f "mybox serve --host 0.0.0.0 --port $PORT" || true)

if [ -n "$PIDS" ]; then
    echo "==> Killing existing mybox serve: $PIDS"
    kill $PIDS

    sleep 1

    PIDS=$(pgrep -f "mybox serve --host 0.0.0.0 --port $PORT" || true)
    if [ -n "$PIDS" ]; then
        echo "==> Force killing: $PIDS"
        kill -9 $PIDS
    fi
    exit 0
fi

while true; do
    echo "==> Starting mybox..."
    mybox serve --host 0.0.0.0 --port "$PORT"

    echo "==> mybox stopped, restarting..."
    sleep 1
done