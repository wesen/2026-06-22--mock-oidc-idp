#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'Usage: %s DEVCTL_BINARY OUTPUT_DIR [--send-hup]\n' "$0"
}

if [[ $# -lt 2 || $# -gt 3 ]]; then
  usage >&2
  exit 2
fi

devctl_binary=$1
output_dir=$2
action=${3:-}

if [[ ! -x "$devctl_binary" ]]; then
  printf 'devctl binary is not executable: %s\n' "$devctl_binary" >&2
  exit 2
fi
if [[ -n "$action" && "$action" != "--send-hup" ]]; then
  usage >&2
  exit 2
fi

mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
fixture_dir="$output_dir/fixture"
mkdir -p "$fixture_dir"

port=$(
  python3 -c 'import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()'
)

stdout_log="$output_dir/service.stdout.log"
stderr_log="$output_dir/service.stderr.log"
exit_info="$output_dir/service.exit.json"
ready_file="$output_dir/service.ready"
wrapper_log="$output_dir/wrapper.log"
process_log="$output_dir/processes.txt"
request_log="$output_dir/http.txt"

"$devctl_binary" __wrap-service \
  --service lifecycle-fixture \
  --cwd "$fixture_dir" \
  --stdout-log "$stdout_log" \
  --stderr-log "$stderr_log" \
  --exit-info "$exit_info" \
  --ready-file "$ready_file" \
  -- \
  python3 -m http.server "$port" --bind 127.0.0.1 \
  >"$wrapper_log" 2>&1 &
wrapper_pid=$!

cleanup() {
  if [[ -n "${child_pid:-}" ]] && kill -0 "$child_pid" 2>/dev/null; then
    kill -TERM "$child_pid" 2>/dev/null || true
  fi
  if kill -0 "$wrapper_pid" 2>/dev/null; then
    wait "$wrapper_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

deadline=$((SECONDS + 5))
while [[ ! -s "$ready_file" && $SECONDS -lt $deadline ]]; do
  sleep 0.05
done
if [[ ! -s "$ready_file" ]]; then
  printf 'wrapper did not create ready file\n' >&2
  exit 1
fi

child_pid=$(tr -d '[:space:]' <"$ready_file")
{
  printf 'wrapper_pid=%s child_pid=%s port=%s\n' "$wrapper_pid" "$child_pid" "$port"
  ps -o pid,ppid,pgid,sid,stat,tty,cmd -p "$wrapper_pid" -p "$child_pid"
} >"$process_log"

http_ready=false
deadline=$((SECONDS + 5))
while [[ $SECONDS -lt $deadline ]]; do
  if curl --fail --silent --show-error --max-time 1 \
    "http://127.0.0.1:$port/" >"$request_log" 2>>"$stderr_log"; then
    http_ready=true
    break
  fi
  sleep 0.05
done
if [[ "$http_ready" != true ]]; then
  printf 'HTTP fixture did not become ready\n' >&2
  exit 1
fi

if [[ "$action" == "--send-hup" ]]; then
  kill -HUP "$wrapper_pid"
  sleep 0.5
  {
    printf '\nafter_sighup\n'
    ps -o pid,ppid,pgid,sid,stat,tty,cmd -p "$wrapper_pid" -p "$child_pid" || true
  } >>"$process_log"
  set +e
  wait "$wrapper_pid"
  wrapper_status=$?
  set -e
  printf 'wrapper_wait_status=%s\n' "$wrapper_status" >>"$process_log"
else
  sleep 0.5
  {
    printf '\nafter_idle_wait\n'
    ps -o pid,ppid,pgid,sid,stat,tty,cmd -p "$wrapper_pid" -p "$child_pid" || true
  } >>"$process_log"
fi

printf 'Artifacts: %s\n' "$output_dir"
