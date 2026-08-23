#!/bin/sh
set -eu

example_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
trust_file="$example_dir/runtime/caddy-local-root.crt"

docker compose --project-directory "$example_dir" -f "$example_dir/compose.yaml" \
  config --quiet

if [ ! -r "$trust_file" ]; then
  "$example_dir/scripts/02-export-browser-ca.sh" >/dev/null
fi

wait_code() {
  expected=$1
  url=$2
  attempt=1
  while [ "$attempt" -le 36 ]; do
    code=$(curl --cacert "$trust_file" -sS -o /dev/null -w '%{http_code}' "$url" || true)
    if [ "$code" = "$expected" ]; then
      printf 'OK %s %s\n' "$expected" "$url"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  printf 'expected %s from %s, received %s\n' "$expected" "$url" "$code" >&2
  return 1
}

wait_code 200 https://idp.localhost:8443/readyz
wait_code 200 https://idp.localhost:8443/admin
wait_code 401 https://idp.localhost:8443/api/admin/session
wait_code 401 https://idp.localhost:8443/api/widget/pages/overview

headers=$(mktemp)
trap 'rm -f "$headers"' EXIT HUP INT TERM
curl --cacert "$trust_file" -sS -o /dev/null -D "$headers" \
  https://idp.localhost:8443/admin
grep -i '^content-security-policy:' "$headers" >/dev/null

docker compose --project-directory "$example_dir" -f "$example_dir/compose.yaml" \
  exec -T idp curl -fsS http://127.0.0.1:9090/readyz >/dev/null
printf 'OK internal readiness and administration workers\n'
