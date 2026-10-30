#!/bin/sh
# Remove Happy Downloader, and leave the downloads alone.
#
# The downloads are the only thing here that cannot be made again, so they are
# never touched: this removes the program, the settings and the service, and
# says where the files still are.
#
# It reads where things were installed from the folder it is in, not from the
# compiled-in default: an installation that was told to go somewhere else is
# still the installation this script belongs to.

set -eu

# This script lives in the folder it removes, which is the only reliable
# statement about where that folder is.
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PREFIX=$HERE
CONFIG_DIR=${HD_CONFIG_DIR:-$HOME/.config/happy-downloader}
DOWNLOAD_DIR=

if [ -f "$PREFIX/install.conf" ]; then
  # shellcheck disable=SC1091
  . "$PREFIX/install.conf"
fi

if [ -z "$DOWNLOAD_DIR" ] && [ -f "$CONFIG_DIR/config.yml" ]; then
  DOWNLOAD_DIR=$(sed -n 's/^ *download_path: *//p' "$CONFIG_DIR/config.yml" | head -1)
fi

case "$PREFIX" in
  ""|"/"|"$HOME"|"$HOME/")
    echo "refusing to remove $PREFIX" >&2
    exit 1
    ;;
esac

echo "==> stopping the service"
systemctl --user disable --now happy-downloader.service 2>/dev/null || true
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/happy-downloader.service"
systemctl --user daemon-reload 2>/dev/null || true

echo "==> removing the program from $PREFIX"
# The script is running from inside the folder it is deleting, so it deletes
# the contents and then the folder itself, from outside.
cd /
rm -rf "$PREFIX"

echo "==> removing the settings from $CONFIG_DIR"
rm -rf "$CONFIG_DIR"

echo
echo "Removed."
if [ -n "$DOWNLOAD_DIR" ] && [ -d "$DOWNLOAD_DIR" ]; then
  echo "The downloads are still in $DOWNLOAD_DIR — that is on purpose."
fi
