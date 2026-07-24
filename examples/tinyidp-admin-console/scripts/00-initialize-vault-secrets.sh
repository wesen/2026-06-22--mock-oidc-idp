#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$repo_root"

devctl secrets-init --profile admin-console
devctl secrets-fetch --profile admin-console
