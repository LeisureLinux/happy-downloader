#!/bin/sh
# Build the whole thing, in the only order that works.
#
# The frontend is embedded into the binary with go:embed, so the Go compiler
# reads frontend/dist as it is at that moment. A stale dist is not detected:
# the build succeeds and ships the previous version's interface. That happened
# once here — a frontend fix was made, the binary was rebuilt without
# rebuilding the frontend, and the old code went out and was debugged for an
# hour.
#
# Removing dist first turns that into a build failure instead of a silent
# wrong artefact. If the frontend build fails, there is no dist to embed and
# the Go build stops with a clear message.

set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

VERSION=${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}
BUILT_AT=$(date +%s)
OUT=${1:-happy-downloader}

echo "==> building frontend"
rm -rf frontend/dist
(
  cd frontend
  pnpm install --frozen-lockfile
  pnpm build
)

if [ ! -f frontend/dist/index.html ]; then
  echo "frontend build produced no dist/index.html" >&2
  exit 1
fi

echo "==> building binary ($VERSION)"
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerVersion=${VERSION} \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerBuiltAt=${BUILT_AT}" \
  -o "$OUT" main.go

echo "==> $OUT ($VERSION, built $BUILT_AT)"
