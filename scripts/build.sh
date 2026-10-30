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

# The version comes from the tag, and only from the tag.
#
# It used to be settable from the environment, which is how the deployed
# binary ended up reporting 4.10.0 while the newest release was 4.6.0: a
# number typed on a command line, drifting away from what was published. There
# is now one source, and a version that exists nowhere else cannot be built.
#
# A build with no tag is a development build and says so.
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")
VERSION=${VERSION#v}

if [ -n "$(git status --porcelain --untracked-files=no 2>/dev/null)" ]; then
  echo "note: the working tree has uncommitted changes; this is not a release build" >&2
fi

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

# The target architecture has to be stated. It is not defaulted: a default
# means a cross-compile that forgets to set it produces a working binary for
# the wrong machine, and deploying that fails at the far end with "Exec format
# error" — after the transfer, where the error says nothing about the cause.
# This bit twice.
#
#   GOARCH=arm64 GOOS=linux ./scripts/build.sh
if [ -z "${GOARCH:-}" ] || [ -z "${GOOS:-}" ]; then
  echo "GOOS and GOARCH must be set, e.g." >&2
  echo "  GOARCH=arm64 GOOS=linux $0 [output]" >&2
  exit 1
fi
export GOARCH GOOS

# A Windows desktop build has no console.
#
# "-H windowsgui" makes the program a windowed application, so double-clicking
# it does not open a black window that has to stay open for the download to
# keep running. It opens the browser at the address instead, and reports a
# failure in a message box, because there is no console left to report it in.
#
# It can still be run from a command prompt: it attaches to the console it was
# started from and prints there.
LDFLAGS="-s -w \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerVersion=${VERSION} \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerBuiltAt=${BUILT_AT}"

if [ "$GOOS" = "windows" ]; then
  LDFLAGS="$LDFLAGS -H windowsgui"
fi

echo "==> building binary ($VERSION, $GOOS/$GOARCH)"
# The package, not main.go: the per-platform files beside it are part of the
# program, and naming one file would leave them out.
CGO_ENABLED=0 go build -trimpath \
  -ldflags "$LDFLAGS" \
  -o "$OUT" .

echo "==> $OUT ($VERSION, built $BUILT_AT)"
