#!/bin/sh
set -eu

example_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
caddy_volume=tinyidp-local-caddy-pki

if ! docker volume inspect "$caddy_volume" >/dev/null 2>&1; then
  docker volume create \
    --label dev.wesen.purpose=local-caddy-pki \
    --label dev.wesen.retention=manual-delete-only \
    "$caddy_volume" >/dev/null
fi

docker compose --project-directory "$example_dir" -f "$example_dir/compose.yaml" \
  config --quiet
docker compose --project-directory "$example_dir" -f "$example_dir/compose.yaml" \
  up --build -d
