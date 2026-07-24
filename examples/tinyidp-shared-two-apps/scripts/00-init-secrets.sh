#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
exec "$repo_root/dev/scripts/bootstrap-vault-profile.sh" shared-two-apps
