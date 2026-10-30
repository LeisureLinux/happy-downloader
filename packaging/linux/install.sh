#!/bin/sh
# Install Happy Downloader for the current user.
#
# There is nothing to compile and nothing to depend on: the four files in this
# folder are the whole program. This script puts them where they belong, writes
# a configuration that points at them, and starts it as a user service so that
# it keeps running without a terminal attached to it.
#
# Run it as yourself, not with sudo:
#
#   ./install.sh
#
# Nothing here needs root. The program lives under ~/.local, its settings under
# ~/.config, and it is reachable at http://localhost:3033/ afterwards.

set -eu

HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# Where things go. They are overridable so that this can be run in a test
# without touching the machine it runs on.
PREFIX=${HD_PREFIX:-$HOME/.local/share/happy-downloader}
CONFIG_DIR=${HD_CONFIG_DIR:-$HOME/.config/happy-downloader}
BIN_DIR=$PREFIX/bin
DATA_DIR=$PREFIX/data
DOWNLOAD_DIR=${HD_DOWNLOAD_DIR:-$HOME/Downloads/happy-downloader}
PORT=${HD_PORT:-3033}
WITH_SERVICE=1

usage() {
  cat <<'USAGE'
usage: ./install.sh [options]

  --prefix DIR      install the program into DIR   (default ~/.local/share/happy-downloader)
  --config-dir DIR  write the settings into DIR    (default ~/.config/happy-downloader)
  --downloads DIR   download to DIR                (default ~/Downloads/happy-downloader)
  --port N          listen on port N               (default 3033)
  --no-service      do not install or start the user service
  -h, --help        this
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --prefix)      PREFIX=$2; BIN_DIR=$PREFIX/bin; DATA_DIR=$PREFIX/data; shift 2 ;;
    --config-dir)  CONFIG_DIR=$2; shift 2 ;;
    --downloads)   DOWNLOAD_DIR=$2; shift 2 ;;
    --port)        PORT=$2; shift 2 ;;
    --no-service)  WITH_SERVICE=0; shift ;;
    -h|--help)     usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# The application and the two programs it runs. They have to end up in the same
# directory: the application looks for the other two beside itself first, and a
# configured path is a second chance rather than the mechanism.
APP=
for candidate in "$HERE"/happy-downloader_linux-* "$HERE"/happy-downloader; do
  if [ -f "$candidate" ] && [ ! -d "$candidate" ]; then
    APP=$candidate
    break
  fi
done

if [ -z "$APP" ]; then
  echo "the application is not in this folder: $HERE" >&2
  echo "extract the whole archive, then run this script from inside it" >&2
  exit 1
fi

for need in yt-dlp ffmpeg; do
  if [ ! -f "$HERE/$need" ]; then
    echo "$need is not in this folder: $HERE" >&2
    echo "extract the whole archive, then run this script from inside it" >&2
    exit 1
  fi
done

echo "==> installing into $PREFIX"
mkdir -p "$BIN_DIR" "$DATA_DIR" "$CONFIG_DIR" "$DOWNLOAD_DIR"

# Installing while it is running would replace the binary of a running process.
# On Linux that works, but the running copy keeps the old code until it is
# restarted, which looks like the install did nothing.
if systemctl --user is-active --quiet happy-downloader.service 2>/dev/null; then
  echo "    stopping the running copy"
  systemctl --user stop happy-downloader.service || true
fi

# The binary keeps the plain name, because that is what the service runs and
# what a person types. The release's platform name is only for the download.
install -m 755 "$APP" "$BIN_DIR/happy-downloader"
install -m 755 "$HERE/yt-dlp" "$BIN_DIR/yt-dlp"
install -m 755 "$HERE/ffmpeg" "$BIN_DIR/ffmpeg"

if [ -f "$HERE/uninstall.sh" ]; then
  install -m 755 "$HERE/uninstall.sh" "$PREFIX/uninstall.sh"
fi

# What was installed where, so that the uninstaller does not have to guess.
# It used to guess, and guessing wrong removed the default location while the
# real one stayed behind — or removed a default location that belonged to a
# different installation.
cat > "$PREFIX/install.conf" <<EOF
PREFIX=$PREFIX
CONFIG_DIR=$CONFIG_DIR
DOWNLOAD_DIR=$DOWNLOAD_DIR
EOF
echo "    $BIN_DIR/happy-downloader"
echo "    $BIN_DIR/yt-dlp"
echo "    $BIN_DIR/ffmpeg"

# The settings.
#
# The two programs are named by their full path even though the application
# would find them beside itself: this file is the answer to "what is it
# actually running", and a path says it without having to know the search
# order.
CONFIG=$CONFIG_DIR/config.yml

if [ -f "$CONFIG" ]; then
  # An existing configuration belongs to the person who wrote it. It is
  # copied aside rather than replaced, and the new one starts from it.
  cp "$CONFIG" "$CONFIG.before-$(date +%Y%m%d-%H%M%S)"
  echo "==> keeping the old settings beside the new ones"
fi

cat > "$CONFIG" <<EOF
# Written by install.sh on $(date +%Y-%m-%d).
#
# The settings page in the interface can change the download folder, the
# proxy and the PO Token provider; it writes them back here. Everything else
# is meant to be edited by hand.
server:
  host: 0.0.0.0
  port: $PORT

paths:
  download_path: $DOWNLOAD_DIR
  downloader_path: $BIN_DIR/yt-dlp
  ffmpeg_path: $BIN_DIR/ffmpeg
  local_database_path: $DATA_DIR

logging:
  enable_file_logging: true
  log_path: $DATA_DIR/happy-downloader.log
EOF

echo "    $CONFIG"

if [ "$WITH_SERVICE" = "1" ]; then
  UNIT_DIR=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user
  UNIT=$UNIT_DIR/happy-downloader.service

  mkdir -p "$UNIT_DIR"

  cat > "$UNIT" <<EOF
[Unit]
Description=Happy Downloader
Documentation=https://github.com/LeisureLinux/happy-downloader
After=network-online.target

[Service]
Type=simple
WorkingDirectory=$PREFIX
ExecStart=$BIN_DIR/happy-downloader -conf $CONFIG
Restart=on-failure
RestartSec=5
# Nothing here needs a terminal, a session or root.
NoNewPrivileges=yes
PrivateTmp=yes

[Install]
WantedBy=default.target
EOF

  echo "    $UNIT"

  if systemctl --user daemon-reload 2>/dev/null && systemctl --user enable --now happy-downloader.service 2>/dev/null; then
    # Without lingering the service stops when the last session ends, which on
    # a machine used over SSH means it stops the moment you log out.
    if ! loginctl enable-linger "$USER" 2>/dev/null; then
      echo "    note: could not enable lingering; it will stop when you log out"
      echo "          sudo loginctl enable-linger $USER   (once, usually)"
    fi
  else
    echo "!! the user service could not be started here" >&2
    echo "   start it yourself with:" >&2
    echo "     $BIN_DIR/happy-downloader -conf $CONFIG" >&2
  fi
fi

cat <<EOF

Installed.

Open http://localhost:$PORT/ in a browser — that page is the application.

  downloads   $DOWNLOAD_DIR
  settings    $CONFIG
  log         $DATA_DIR/happy-downloader.log

To stop it:   systemctl --user stop happy-downloader
To remove it: $PREFIX/uninstall.sh
EOF
