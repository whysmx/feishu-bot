#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BOT_BIN="${BOT_BIN:-$ROOT/bin/bot}"
PID_FILE="${BOT_PID_FILE:-/tmp/feishu-bot.pid}"

collect_pids=()
running_bot_pids=()

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

collect_orphan_go_build_pids() {
  local ips=("$@")
  local pid cmd comm
  for ip in "${ips[@]}"; do
    while read -r pid; do
      [[ -z "$pid" ]] && continue
      cmd="$(ps -p "$pid" -o command=)"
      comm="$(ps -p "$pid" -o comm=)"
      if [[ "$cmd" == "$BOT_BIN"* || "$cmd" == "$ROOT/bot"* ]]; then
        continue
      fi
      if [[ "$comm" == "main" && "$cmd" == *"/go-build"*"/main"* ]]; then
        collect_pids+=("$pid")
      fi
    done < <(lsof -n -P -iTCP -sTCP:ESTABLISHED 2>/dev/null | awk -v ip="$ip" 'NR>1 && $0 ~ "->"ip":443" {print $2}')
  done
}

if [[ -f "$PID_FILE" ]]; then
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "${pid:-}" ]] && ps -p "$pid" > /dev/null 2>&1; then
    cmd="$(ps -p "$pid" -o command=)"
    if [[ "$cmd" == "$BOT_BIN"* || "$cmd" == "$ROOT/bot"* ]]; then
      collect_pids+=("$pid")
      running_bot_pids+=("$pid")
    fi
  fi
  rm -f "$PID_FILE"
fi

for pid in $(pgrep -f "$BOT_BIN" || true); do
  collect_pids+=("$pid")
  running_bot_pids+=("$pid")
done

if command -v lsof >/dev/null 2>&1; then
  go_run_pids="$(ps -ax -o pid=,comm= | awk '$2=="main"{print $1}')"
  for pid in $go_run_pids; do
    cwd="$(lsof -a -d cwd -p "$pid" 2>/dev/null | awk 'NR==2{print $NF}')"
    if [[ "$cwd" == "$ROOT"* ]]; then
      collect_pids+=("$pid")
    fi
  done

  feishu_ips=()
  if (( ${#running_bot_pids[@]} )); then
    for pid in $(printf '%s\n' "${running_bot_pids[@]}" | sort -u); do
      ip="$(lsof -n -P -iTCP -sTCP:ESTABLISHED -a -p "$pid" 2>/dev/null | awk 'NR>1{split($9,a,"->"); if (length(a)>1){split(a[2],b,":"); if (b[2]=="443"){print b[1]; exit}}}')"
      if [[ -n "${ip:-}" ]]; then
        feishu_ips+=("$ip")
        break
      fi
    done
  fi
  if [[ ${#feishu_ips[@]} -eq 0 ]]; then
    while IFS= read -r ip; do
      [[ -n "$ip" ]] && feishu_ips+=("$ip")
    done < <(resolve_msg_frontier_ips)
  fi
  if [[ ${#feishu_ips[@]} -gt 0 ]]; then
    collect_orphan_go_build_pids "${feishu_ips[@]}"
  fi
fi

if [[ ${#collect_pids[@]} -eq 0 ]]; then
  echo "No bot process found."
  exit 0
fi

unique_pids="$(printf '%s\n' "${collect_pids[@]}" | sort -u | tr '\n' ' ')"
for pid in $unique_pids; do
  if ps -p "$pid" > /dev/null 2>&1; then
    echo "Stopping pid=$pid"
    kill "$pid" || true
  fi
done

sleep 1
for pid in $unique_pids; do
  if ps -p "$pid" > /dev/null 2>&1; then
    echo "Force killing pid=$pid"
    kill -9 "$pid" || true
  fi
done
