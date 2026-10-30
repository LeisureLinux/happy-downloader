#!/bin/bash
# The bundle: the application and the programs it needs, in one folder.
#
# A release that ships only the binary asks the person installing it to
# install Python and a media toolkit as well, and to write down where they
# ended up. The bundle is the answer to that: it is a folder that can be
# copied, and the application finds its programs beside itself.
#
# What goes in it:
#
#   happy-downloader     the application
#   yt-dlp               the downloader, as a standalone binary
#   ffmpeg               the media toolkit, for merging and for remuxing
#   README.txt           what it is, and where the programs came from
#
# The POT provider is not in it. It is a Node program of its own, it wants a
# Node runtime beside it as well, and it is optional: without it, only the
# videos that YouTube serves to a guest are refused. A bundle that carried it
# would be three times the size for something most installs do not need, and
# the instructions for setting it up are two lines.
#
# The programs are fetched rather than built, so the bundle is the same on
# every platform the application is built for, and so the bundle does not
# need a Python or a compiler on the machine that makes it.

set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${1:-dist-bundle}
PLATFORM=${2:-linux}
ARCH=${3:-amd64}

mkdir -p "$OUT"

# An executable on Windows carries the suffix.
exe_suffix=""
case "$PLATFORM" in
  windows) exe_suffix=".exe" ;;
esac

# The application.
#
# The name is the one the release publishes, so that the binary in the
# bundle and the binary published beside it are the same file.
case "$PLATFORM" in
  windows) APP="happy-downloader_windows-$ARCH.exe" ;;
  *)      APP="happy-downloader_linux-$ARCH" ;;
esac

if [ ! -f "$ROOT/$APP" ]; then
  echo "the application binary was not found: $ROOT/$APP" >&2
  echo "build it first: GOOS=$PLATFORM GOARCH=$ARCH ./scripts/build.sh $APP" >&2
  exit 1
fi

cp "$ROOT/$APP" "$OUT/"

# yt-dlp, as a standalone binary. The one that needs no Python.
case "$PLATFORM" in
  linux)
    case "$ARCH" in
      arm64) YTDLP=yt-dlp_linux_aarch64 ;;
      *)     YTDLP=yt-dlp_linux ;;
    esac
    ;;
  windows)
    case "$ARCH" in
      arm64) YTDLP=yt-dlp_windows_arm64.exe ;;
      *)     YTDLP=yt-dlp.exe ;;
    esac
    ;;
  *)
    echo "unknown platform: $PLATFORM" >&2
    exit 1
    ;;
esac

# The release names it with the platform in it, so that the downloads do not
# collide with each other. The bundle has one platform in it, and the
# application looks for the plain name beside itself, so it is stored under
# that.
echo "==> fetching yt-dlp ($YTDLP)"
curl -sL -f -o "$OUT/yt-dlp${exe_suffix}" "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$YTDLP"
chmod +x "$OUT/yt-dlp${exe_suffix}"

# ffmpeg. The build that is one file and needs nothing beside it.
# The names are the ones the release uses, which are not the same as the
# architecture names: x64 is amd64, and there is no Windows arm64 build.
case "$PLATFORM" in
  linux)
    case "$ARCH" in
      arm64) FFMPEG=ffmpeg-linux-arm64 ;;
      *)     FFMPEG=ffmpeg-linux-x64 ;;
    esac
    ;;
  windows)
    # Only amd64. There is no Windows arm64 build of ffmpeg to put beside
    # the application, and a bundle without ffmpeg is not a bundle.
    if [ "$ARCH" != "amd64" ]; then
      echo "there is no Windows $ARCH bundle: ffmpeg is not built for it" >&2
      exit 1
    fi
    FFMPEG=ffmpeg-win32-x64
    ;;
esac

echo "==> fetching ffmpeg ($FFMPEG)"
FFMPEG_URL="https://github.com/eugeneware/ffmpeg-static/releases/latest/download/$FFMPEG"
if ! curl -sL -f -o "$OUT/ffmpeg${exe_suffix}" "$FFMPEG_URL"; then
  echo "    ffmpeg could not be fetched from $FFMPEG_URL" >&2
  exit 1
fi
chmod +x "$OUT/ffmpeg${exe_suffix}"

cat > "$OUT/README.txt" <<'EOF'
Happy Downloader — the bundle
=============================

Everything the downloader needs is in this folder:

  happy-downloader    the application
  yt-dlp              the downloader
  ffmpeg              for merging streams and for putting a recording into
                      a container that plays

Put the folder wherever you like and run it. It finds the other two beside
itself, so there is nothing to configure and nothing to install.

Windows
-------

Double-click happy-downloader. There is no window and no command prompt to
keep open: it starts, it opens your browser at

  http://localhost:3033/

and that page is the application. Closing the browser does not stop it —
bookmarking the address is the usual thing to do — and if you want it gone,
the menu in the page has a quit entry.

Downloads go to "Downloads\happy-downloader" under your user folder, which
you can change on the settings page. Nothing needs administrator rights: the
database, the settings and the log live in

  %APPDATA%\happy-downloader

If you would rather see the log in a terminal, run it from one:
  happy-downloader_windows-amd64.exe -conf config.yml
It attaches to the console it was started from and prints there.

First run
---------

It wants to know two things, which it asks for on the first run and remembers:

  where the downloads go
  which cookies to use

A cookie export is a browser extension's export file. It is what makes the
sites that need a signed-in session work; without one, those sites answer as
a guest and refuse.

The PO Token provider is not here
--------------------------------

YouTube asks some downloads for a token that proves the request came from a
real browser. The provider that produces it is a separate program that needs
Node beside it, and it is optional: without it, only the videos YouTube
serves to a guest are refused.

To add it, see https://github.com/Brainicism/bgutil-ytdlp-pot-provider
and point ytdlp.pot_base_url at it.
EOF

echo
echo "the bundle:"
for file in "$OUT"/*; do
  printf "  %-34s %s\n" "$(basename "$file")" "$(du -h "$file" | cut -f1)"
done
