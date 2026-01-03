#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
else
  echo "WARN: .env not found; continuing without environment overrides." >&2
fi

BOT_BIN="${BOT_BIN:-$ROOT/bin/bot}"
PID_FILE="${BOT_PID_FILE:-/tmp/feishu-bot.pid}"
if [[ ! -x "$BOT_BIN" ]]; then
  if [[ -x "$ROOT/bot" ]]; then
    BOT_BIN="$ROOT/bot"
  else
    echo "ERROR: bot binary not found. Build it first." >&2
    exit 1
  fi
fi

LOG_FILE="${LOG_FILE:-/tmp/feishu-bot-latest.log}"

resolve_msg_frontier_ips() {
  if command -v dig >/dev/null 2>&1; then
    dig +short msg-frontier.feishu.cn 2>/dev/null | awk '/^[0-9.]+$/'
    return
  fi
  if command -v host >/dev/null 2>&1; then
    host msg-frontier.feishu.cn 2>/dev/null | awk '/has address/ {print $4}'
    return
  fi
  if command -v nslookup >/dev/null 2>&1; then
    nslookup msg-frontier.feishu.cn 2>/dev/null | awk 'NR>1 && $1=="Address:" {print $2}'
  fi
}

find_orphan_go_build_pids() {
  local ips=("$@")
  local pid cmd comm
  local found=()
  for ip in "${ips[@]}"; do
    while read -r pid; do
      [[ -z "$pid" ]] && continue
      cmd="$(ps -p "$pid" -o command=)"
      comm="$(ps -p "$pid" -o comm=)"
      if [[ "$cmd" == "$BOT_BIN"* || "$cmd" == "$ROOT/bot"* ]]; then
        continue
      fi
      if [[ "$comm" == "main" && "$cmd" == *"/go-build"*"/main"* ]]; then
        found+=("$pid")
      fi
    done < <(lsof -n -P -iTCP -sTCP:ESTABLISHED 2>/dev/null | awk -v ip="$ip" 'NR>1 && $0 ~ "->"ip":443" {print $2}')
  done
  if (( ${#found[@]} )); then
    printf '%s\n' "${found[@]}" | sort -u
  fi
}

if [[ -f "$PID_FILE" ]]; then
  existing_pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "${existing_pid:-}" ]] && ps -p "$existing_pid" > /dev/null 2>&1; then
    existing_cmd="$(ps -p "$existing_pid" -o command=)"
    if [[ "$existing_cmd" == "$BOT_BIN"* ]]; then
      echo "ERROR: bot already running (pid=$existing_pid)." >&2
      exit 1
    fi
  fi
  rm -f "$PID_FILE"
fi

other_pids="$(pgrep -f "$BOT_BIN" || true)"
if [[ -n "$other_pids" ]]; then
  echo "ERROR: found existing bot process(es): $other_pids" >&2
  echo "Run ./scripts/stop-bot.sh first." >&2
  exit 1
fi

if command -v lsof >/dev/null 2>&1; then
  go_run_pids="$(ps -ax -o pid=,comm= | awk '$2=="main"{print $1}')"
  for pid in $go_run_pids; do
    cwd="$(lsof -a -d cwd -p "$pid" 2>/dev/null | awk 'NR==2{print $NF}')"
    if [[ "$cwd" == "$ROOT"* ]]; then
      echo "ERROR: found go run instance (pid=$pid) in repo. Stop it first." >&2
      exit 1
    fi
  done

  feishu_ips=()
  while IFS= read -r ip; do
    [[ -n "$ip" ]] && feishu_ips+=("$ip")
  done < <(resolve_msg_frontier_ips)
  if [[ ${#feishu_ips[@]} -gt 0 ]]; then
    orphan_pids=()
    while IFS= read -r pid; do
      [[ -n "$pid" ]] && orphan_pids+=("$pid")
    done < <(find_orphan_go_build_pids "${feishu_ips[@]}")
    if [[ ${#orphan_pids[@]} -gt 0 ]]; then
      echo "WARN: found orphan go-build Feishu connections: ${orphan_pids[*]}" >&2
      for pid in "${orphan_pids[@]}"; do
        echo "Stopping orphan pid=$pid" >&2
        kill "$pid" || true
      done
      sleep 1
      for pid in "${orphan_pids[@]}"; do
        if ps -p "$pid" > /dev/null 2>&1; then
          echo "Force killing orphan pid=$pid" >&2
          kill -9 "$pid" || true
        fi
      done
    fi
  fi
fi

nohup "$BOT_BIN" > "$LOG_FILE" 2>&1 &
echo "$!" > "$PID_FILE"
echo "Bot PID: $!"
