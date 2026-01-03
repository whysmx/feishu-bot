#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

./scripts/stop-bot.sh

if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
  go build -a -o bin/bot cmd/bot/main.go
fi

./scripts/start-bot.sh
