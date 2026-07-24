#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  printf 'usage: %s <devctl-profile>\n' "$0" >&2
  exit 2
fi

profile=$1
caddy_volume=tinyidp-local-caddy-pki

case "$profile" in
  admin-console|shared-two-apps|jitsi) ;;
  *)
    printf 'profile %s does not use the shared local Caddy authority\n' "$profile" >&2
    exit 2
    ;;
esac

if ! docker volume inspect "$caddy_volume" >/dev/null 2>&1; then
  docker volume create \
    --label dev.wesen.purpose=local-caddy-pki \
    --label dev.wesen.retention=manual-delete-only \
    "$caddy_volume" >/dev/null
  printf 'Created persistent local Caddy PKI volume %s\n' "$caddy_volume"
fi

devctl --profile "$profile" secrets-init
devctl --profile "$profile" secrets-fetch
printf 'Prepared Vault-backed secrets for profile %s\n' "$profile"
