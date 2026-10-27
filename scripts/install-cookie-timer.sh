#!/bin/sh
# Install the cookie-refresh timer, naming this checkout by absolute path.
#
# A user unit starts with no working directory, so the unit has to spell out
# where the script is. That makes it a copy of the repository's location, and a
# copy goes stale the moment the directory is renamed — silently: the timer
# keeps firing, the service fails with 203/EXEC, and the only symptom is that
# the cookie file's timestamp stops moving while downloads start failing with a
# bot wall.
#
# So the unit is generated here rather than edited by hand, and re-running this
# after moving the checkout is the whole repair.
#
# Usage:
#   install-cookie-timer.sh [--deploy user@host:/path] [--output <path>]
#                           [--browser chrome+gnomekeyring] [--dry-run]
#
# --deploy is for the usual layout: the browser is on this machine, the server
# that downloads is another one. Omit it when this host does both.
#
# --output defaults to %h/.config/happy-downloader/youtube-cookie.json, which is
# what the server's paths.cookie_path points at.

set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEMPLATE_DIR="$ROOT/packaging/systemd"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

OUTPUT="%h/.config/happy-downloader/youtube-cookie.json"
BROWSER="chrome"
DEPLOY=""
DRY_RUN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --deploy)  DEPLOY="${2:-}"; shift 2 ;;
    --output)  OUTPUT="${2:-}"; shift 2 ;;
    --browser) BROWSER="${2:-}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -n "$DEPLOY" ]; then
  case "$DEPLOY" in
    *@*:*) : ;;
    *) echo "--deploy wants user@host:/path, got: $DEPLOY" >&2; exit 2 ;;
  esac
  DEPLOY_ARG="--deploy $DEPLOY"
else
  DEPLOY_ARG=""
fi

command -v systemctl >/dev/null 2>&1 || { echo "systemctl not found" >&2; exit 1; }

render() {
  sed -e "s|@REPO@|$ROOT|g" \
      -e "s|@OUTPUT@|$OUTPUT|g" \
      -e "s|@BROWSER@|$BROWSER|g" \
      -e "s|@DEPLOY_ARG@|$DEPLOY_ARG|g" \
      "$1"
}

SERVICE="$UNIT_DIR/happy-downloader-cookies.service"
TIMER="$UNIT_DIR/happy-downloader-cookies.timer"

if [ "$DRY_RUN" -eq 1 ]; then
  echo "would write $SERVICE:"
  render "$TEMPLATE_DIR/happy-downloader-cookies.service"
  echo
  echo "would write $TIMER and enable happy-downloader-cookies.timer"
  exit 0
fi

mkdir -p "$UNIT_DIR"
render "$TEMPLATE_DIR/happy-downloader-cookies.service" > "$SERVICE"
render "$TEMPLATE_DIR/happy-downloader-cookies.timer" > "$TIMER"

systemctl --user daemon-reload
systemctl --user enable --now happy-downloader-cookies.timer

# Say what the unit now points at, and whether the last run worked. A timer
# that is enabled and failing looks identical to a working one in a list.
echo
systemctl --user list-timers happy-downloader-cookies.timer --no-pager || true
echo
if systemctl --user is-active --quiet happy-downloader-cookies.service; then
  echo "the service is running now; watch it with:"
else
  echo "run it once, then read the log:"
fi
echo "  systemctl --user start happy-downloader-cookies.service"
echo "  journalctl --user -u happy-downloader-cookies.service -n 30"
