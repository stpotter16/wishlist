#!/usr/bin/env bash

# Fail on first error
set -e

# Fail on unset variable
set -u

# SEED=demo fills the freshly deployed database with demo households. Leave it
# unset for a blank slate.
SEED="${SEED:-}"
readonly SEED

if [[ -n "${SEED}" && "${SEED}" != 'demo' ]]; then
  echo "Unknown SEED value '${SEED}'. Use SEED=demo or leave it unset." >&2
  exit 1
fi

# Each Fly machine has its own SQLite file, so run exactly one.
fly deploy --ha=false

if [[ "${SEED}" == 'demo' ]]; then
  fly ssh console --command '/app/wishlist-admin seed-demo'
fi
