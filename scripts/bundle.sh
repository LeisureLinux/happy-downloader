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

# Two program files, the two programs they need, and the thing that puts them
# in place. On Linux the bundle is run; on Windows it is unpacked by an
# installer instead, so the scripts are not copied there.
if [ "$PLATFORM" = "linux" ]; then
  cp "$ROOT/packaging/linux/install.sh" "$OUT/install.sh"
  cp "$ROOT/packaging/linux/uninstall.sh" "$OUT/uninstall.sh"
  chmod +x "$OUT/install.sh" "$OUT/uninstall.sh"
fi

# What is in here, and what to do with it, is different enough between the two
# platforms to be two files rather than one with a paragraph of exceptions.
if [ "$PLATFORM" = "linux" ]; then

cat > "$OUT/README.txt" <<'EOF'
Happy Downloader — the bundle
=============================

Everything the downloader needs is in this folder:

  happy-downloader_linux-*  the application
  yt-dlp                    the downloader
  ffmpeg                    for merging streams and for putting a recording
                            into a container that plays
  install.sh                puts them where they belong
  uninstall.sh              takes them away again, and leaves the downloads

Install
-------

    ./install.sh

It needs no root and no dependencies. It copies the program into
~/.local/share/happy-downloader, writes a configuration naming the two
programs it just installed, and starts it as a user service so that it keeps
running without a terminal attached.

Open http://localhost:3033/ in a browser afterwards — that page is the
application. Nothing else to do.

  --prefix DIR      install somewhere else
  --downloads DIR   download somewhere else
  --port N          listen on another port
  --no-service      install the files, do not run it

Without a service
-----------------

    ./install.sh --no-service
    ~/.local/share/happy-downloader/bin/happy-downloader \
        -conf ~/.config/happy-downloader/config.yml

The PO Token provider is not here
---------------------------------

YouTube asks some downloads for a token that proves the request came from a
real browser. The provider that produces it is a separate program that needs
Node beside it, and it is optional: without it, only the videos YouTube
serves to a guest are refused.

To add it, see https://github.com/Brainicism/bgutil-ytdlp-pot-provider
and point ytdlp.pot_base_url at it.
EOF

else

cat > "$OUT/README.txt" <<'EOF'
Happy Downloader — the files, for testing
=========================================

This zip is the application with the two programs it runs, and nothing else.
It is what the release builds to check that the three of them work together.

If you are installing this for real, use
happy-downloader_windows-amd64-setup.exe instead: it asks where to put the
program, writes the configuration, and makes a shortcut.

Running this one anyway
-----------------------

Double-click happy-downloader_windows-amd64.exe. There is no console window:
it starts, it opens your browser at

  http://localhost:3033/

and that page is the application.

It looks for yt-dlp.exe and ffmpeg.exe beside itself, which is how the two
files in this folder are found. Keep them together.

Downloads go to "Downloads\happy-downloader" under your user folder; the
settings page can change that. The database, the settings and the log live in

  %APPDATA%\happy-downloader

To see the log in a terminal, run it from one — it attaches to the console it
was started from and prints there instead of opening a browser:

  happy-downloader_windows-amd64.exe

Windows asks once whether to let it accept connections from the network. It
does not need that to work on this machine; allow it if you want to open the
page from your phone.

The PO Token provider is not here
---------------------------------

YouTube asks some downloads for a token that proves the request came from a
real browser. The provider that produces it is a separate program that needs
Node beside it, and it is optional: without it, only the videos YouTube
serves to a guest are refused.

To add it, see https://github.com/Brainicism/bgutil-ytdlp-pot-provider
and point ytdlp.pot_base_url at it.
EOF

fi

echo
echo "the bundle:"
for file in "$OUT"/*; do
  printf "  %-34s %s\n" "$(basename "$file")" "$(du -h "$file" | cut -f1)"
done
