#!/usr/bin/env bash

# Fail on first error
set -e

# Fail on unset variable
set -u

# Each Fly machine has its own SQLite file, so run exactly one.
fly deploy --ha=false
