#!/usr/bin/env bash
# Run a go command in the golang container (no local Go toolchain on the build host).
exec docker run --rm --memory 4g -u "$(id -u):$(id -g)" \
  -e GOCACHE=/cache/build -e GOMODCACHE=/cache/mod -e HOME=/tmp \
  -v "$HOME/.cache/go-docker:/cache" -v "$(cd "$(dirname "$0")/.." && pwd):/src" -w /src \
  golang:1.26 go "$@"
