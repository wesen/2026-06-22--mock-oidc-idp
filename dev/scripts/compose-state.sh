#!/bin/sh
set -eu

if [ "$#" -lt 3 ]; then
  printf 'usage: %s <profile> <compose-file> <status|reset|reset-local> [--confirm reset-<profile>-state]\n' "$0" >&2
  exit 2
fi

profile=$1
compose_file=$2
operation=$3
shift 3

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
case "$compose_file" in
  /*|*".."*)
    printf 'compose file must be a repository-relative path without .. segments\n' >&2
    exit 2
    ;;
esac

compose_path="$repo_root/$compose_file"
if [ ! -f "$compose_path" ]; then
  printf 'compose file does not exist: %s\n' "$compose_file" >&2
  exit 2
fi

case "$operation" in
  status)
    if [ "$#" -ne 0 ]; then
      printf 'state-status does not accept arguments\n' >&2
      exit 2
    fi
    exec docker compose -f "$compose_path" ps -a
    ;;
  reset|reset-local)
    expected="reset-$profile-state"
    if [ "$#" -ne 2 ] || [ "$1" != "--confirm" ] || [ "$2" != "$expected" ]; then
      printf 'refusing state reset; rerun with --confirm %s\n' "$expected" >&2
      exit 2
    fi

    caddy_volume=tinyidp-local-caddy-pki
    before=
    if [ "$operation" = "reset" ]; then
      if ! docker compose -f "$compose_path" config --format json |
        jq -e '.volumes["caddy-data"].external == true and .volumes["caddy-data"].name == "tinyidp-local-caddy-pki"' >/dev/null
      then
        printf 'refusing state reset; Compose does not declare the protected external Caddy volume\n' >&2
        exit 2
      fi
      if docker volume inspect "$caddy_volume" >/dev/null 2>&1; then
        before=$(docker volume inspect --format '{{.CreatedAt}}|{{.Mountpoint}}' "$caddy_volume")
      fi
    fi

    docker compose -f "$compose_path" down --volumes --remove-orphans

    if [ -n "$before" ]; then
      after=$(docker volume inspect --format '{{.CreatedAt}}|{{.Mountpoint}}' "$caddy_volume")
      if [ "$after" != "$before" ]; then
        printf 'external Caddy PKI volume identity changed during reset\n' >&2
        exit 1
      fi
    fi
    if [ "$operation" = "reset" ]; then
      printf 'Reset application state for profile %s; retained external Caddy PKI volume\n' "$profile"
    else
      printf 'Reset local application state for profile %s\n' "$profile"
    fi
    ;;
  *)
    printf 'unknown state operation: %s\n' "$operation" >&2
    exit 2
    ;;
esac
