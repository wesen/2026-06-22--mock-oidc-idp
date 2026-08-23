#!/bin/sh
set -eu

example_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mkdir -p "$example_dir/runtime"

docker compose --project-directory "$example_dir" -f "$example_dir/compose.yaml" \
  cp ca-export:/trust/caddy-local-root.crt \
  "$example_dir/runtime/caddy-local-root.crt"

chmod 0644 "$example_dir/runtime/caddy-local-root.crt"
openssl x509 -in "$example_dir/runtime/caddy-local-root.crt" \
  -noout -fingerprint -sha256
