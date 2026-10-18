#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo "ERROR: $PROJECT_ROOT is not a git repository" >&2
    exit 1
fi

echo "==> Pulling latest changes..."
git -C "$PROJECT_ROOT" pull

echo "==> Installing mybox..."
cd "$PROJECT_ROOT/agenttools/mybox"
make install

echo "mybox has been re-installed."
