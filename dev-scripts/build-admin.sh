#!/usr/bin/env bash

# Fail on first error
set -e

# Fail on unset variable
set -u

# Echo commands
set -x

go build -o ./tmp/admin cmd/admin/main.go

