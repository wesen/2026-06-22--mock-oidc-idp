#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  printf 'Usage: %s DEVCTL_BINARY PLUGIN_PATH OUTPUT_DIR\n' "$0" >&2
  exit 2
fi

devctl_binary=$1
plugin_path=$2
output_dir=$3

if [[ ! -x "$devctl_binary" ]]; then
  printf 'devctl binary is not executable: %s\n' "$devctl_binary" >&2
  exit 2
fi
if [[ ! -f "$plugin_path" ]]; then
  printf 'plugin does not exist: %s\n' "$plugin_path" >&2
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

config_path="$fixture_dir/.devctl.yaml"
printf 'plugins:\n  - id: lifecycle\n    path: python3\n    args:\n      - "%s"\n      - "%s"\n    priority: 10\n' \
  "$plugin_path" "$port" >"$config_path"

cleanup() {
  "$devctl_binary" down --repo-root "$fixture_dir" \
    >>"$output_dir/down.log" 2>&1 || true
}
trap cleanup EXIT

"$devctl_binary" up --repo-root "$fixture_dir" \
  >"$output_dir/up.stdout.log" 2>"$output_dir/up.stderr.log"

for delay in 0 1 5; do
  if [[ "$delay" -gt 0 ]]; then
    sleep "$delay"
  fi
  {
    printf 'delay_seconds=%s\n' "$delay"
    "$devctl_binary" status --repo-root "$fixture_dir" || true
    wrapper_pid=$(
      python3 -c 'import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f)["services"][0]["pid"])' \
        "$fixture_dir/.devctl/state.json"
    )
    ps -o pid,ppid,pgid,sid,stat,tty,cmd -p "$wrapper_pid" || true
    pgrep -a -P "$wrapper_pid" || true
    curl --fail --silent --show-error --max-time 2 \
      "http://127.0.0.1:$port/" >/dev/null && printf 'http=ready\n' || printf 'http=failed\n'
    printf '\n'
  } >>"$output_dir/timeline.txt" 2>&1
done

cp -R "$fixture_dir/.devctl" "$output_dir/devctl-state"
printf 'Artifacts: %s\n' "$output_dir"
