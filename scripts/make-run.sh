#!/bin/sh
# Wrap a staged directory into one self-extracting installer.
#
# One file, run it, done. It is the Linux answer to a setup program: an archive
# is not an installation, and asking somebody to unpack four files, put them
# somewhere and then point a configuration at them is asking them to do the
# installer's job.
#
#   ./make-run.sh <staged-dir> <output-file> [version]
#
# The result needs no root, no dependencies and no Python: it is a shell script
# with a tar archive after it.

set -eu

STAGE=${1:?usage: make-run.sh <staged-dir> <output-file> [version]}
OUT=${2:?usage: make-run.sh <staged-dir> <output-file> [version]}
VERSION=${3:-dev}

if [ ! -d "$STAGE" ]; then
  echo "no such directory: $STAGE" >&2
  exit 1
fi

if [ ! -f "$STAGE/install.sh" ]; then
  echo "the staged directory has no install.sh: $STAGE" >&2
  exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# The header is the whole interface. Everything after the marker is the
# archive, and the marker line is what tells the script where it starts.
cat > "$TMP/header" <<'HEADER'
#!/bin/sh
# Happy Downloader — installer
#
# One file: run it, and the application is installed for the current user and
# started. Nothing here needs root, an internet connection, or anything that is
# not already on the machine.
#
#   ./happy-downloader_*_linux-*.run              install
#   ./happy-downloader_*_linux-*.run --prefix DIR install somewhere else
#   ./happy-downloader_*_linux-*.run --no-service install without a service
#   ./happy-downloader_*_linux-*.run --help       what the options are
#
# Everything it is given is passed to the installer inside, which prints the
# same options.

set -eu

SELF=$0

# The payload starts on the line after the marker. It is found rather than
# assumed, because the size of this header is not something to hard-code.
LINE=$(awk '/^__PAYLOAD_BELOW__$/ { print NR + 1; exit }' "$SELF")

if [ -z "${LINE:-}" ]; then
  echo "this file is not a Happy Downloader installer: the payload marker is missing" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/happy-downloader.XXXXXXXX")

cleanup() {
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

tail -n +"$LINE" "$SELF" | tar -xzf - -C "$WORK"

if [ ! -x "$WORK/happy-downloader/install.sh" ]; then
  echo "the installer did not unpack properly" >&2
  exit 1
fi

status=0
"$WORK/happy-downloader/install.sh" "$@" || status=$?

# Nothing is left behind, whether it worked or not.
rm -rf "$WORK"
trap - EXIT INT TERM

exit $status
__PAYLOAD_BELOW__
HEADER

# The payload is packed from the staged directory, under one folder, so that
# the installer inside finds everything beside itself.
mkdir -p "$TMP/payload"
cp -a "$STAGE" "$TMP/payload/happy-downloader"

{
  cat "$TMP/header"
  (cd "$TMP/payload" && tar -czf - happy-downloader)
} > "$OUT"

chmod +x "$OUT"

echo "==> $OUT ($VERSION, $(du -h "$OUT" | cut -f1))"
