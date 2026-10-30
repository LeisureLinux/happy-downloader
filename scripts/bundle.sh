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

# The application.
#
# The name is the one the release publishes, so that the binary in the
# bundle and the binary published beside it are the same file.
APP="happy-downloader_linux-$ARCH"

if [ ! -f "$ROOT/$APP" ]; then
  echo "the application binary was not found: $ROOT/$APP" >&2
  echo "build it first: GOOS=linux GOARCH=$ARCH ./scripts/build.sh $APP" >&2
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

echo "==> fetching yt-dlp ($YTDLP)"
curl -sL -f -o "$OUT/$YTDLP" "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$YTDLP"
chmod +x "$OUT/$YTDLP"

# It is stored under the plain name, because that is the name the
# application looks for beside itself. The release names it with the
# platform in it so that the downloads do not collide; the bundle has one
# platform in it and does not need the distinction.
mv "$OUT/$YTDLP" "$OUT/yt-dlp"

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
    case "$ARCH" in
      arm64)
        echo "there is no Windows arm64 build of ffmpeg to fetch" >&2
        exit 1
        ;;
      *)     FFMPEG=ffmpeg-win32-x64 ;;
    esac
    ;;
esac

echo "==> fetching ffmpeg ($FFMPEG)"
FFMPEG_URL="https://github.com/eugeneware/ffmpeg-static/releases/latest/download/$FFMPEG"
if ! curl -sL -f -o "$OUT/ffmpeg" "$FFMPEG_URL"; then
  echo "    ffmpeg could not be fetched from $FFMPEG_URL" >&2
  exit 1
fi
chmod +x "$OUT/ffmpeg"

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
