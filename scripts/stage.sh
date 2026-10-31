#!/bin/bash
# Stage what an installer needs: the application and the programs it runs.
#
# This is not a distribution artefact and is not published. It is the input to
# one: the Windows installer is packed from exactly these files, so a release
# cannot ship an installer built from something other than what was tested.
#
# On Linux it is only used by the tests, which start the application from a
# directory like this one to check that it finds the programs beside itself.
# Nothing is published for Linux.
#
# What goes in it:
#
#   happy-downloader     the application
#   yt-dlp               the downloader, as a standalone binary
#   ffmpeg               the media toolkit, for merging and for remuxing
#   README.txt           what it is, and where the programs came from
#   happy-downloader.ico the icon the Windows installer and shortcuts wear
#
# The POT provider is not in it. It is a Node program of its own, it wants a
# Node runtime beside it as well, and it is optional: without it, only the
# videos that YouTube serves to a guest are refused. Carrying it would triple
# the size for something most installs do not need, and the instructions for
# setting it up are two lines.
#
# The programs are fetched rather than built, so the staging is the same on
# every machine that makes it, and it needs no Python or compiler.

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
    YTDLP=yt-dlp.exe
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
    # x64 only: there is no ffmpeg-static build for Windows on ARM.
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

# The Windows installer needs the icon as a file: it is embedded in the
# executable at build time, and the wizard and the shortcuts read it from here.
if [ "$PLATFORM" = "windows" ]; then
  cp "$ROOT/packaging/windows/happy-downloader.ico" "$OUT/"

  # A WebView2 runtime to carry, for machines that have none.
  #
  # Windows 10 and 11 have the Evergreen runtime and it updates itself. Windows
  # 7 does not and cannot install one any more — the last version that runs
  # there is 109 — so an installer that is expected to work there has to carry
  # that runtime itself. It is over 250 MB, which is why it is optional here
  # rather than always: a build without it still works on every machine that
  # already has WebView2.
  #
  # The folder is not in this repository: it is Microsoft's build, downloaded
  # from their fixed-version page and unpacked into
  # packaging/windows/webview2/ (see the README there).
  if [ -d "$ROOT/packaging/windows/webview2" ]; then
    if [ ! -f "$ROOT/packaging/windows/webview2/msedgewebview2.exe" ]; then
      echo "packaging/windows/webview2/ is present but holds no msedgewebview2.exe" >&2
      exit 1
    fi

    echo "==> carrying the bundled WebView2 runtime"
    mkdir -p "$OUT/webview2"
    cp -a "$ROOT/packaging/windows/webview2/." "$OUT/webview2/"
  else
    echo "    no bundled WebView2 runtime: this build needs one on the machine"
  fi
fi

# The README is installed beside the program, so its reader has already got
# past the installing. It says where things are and how to get rid of them.
cat > "$OUT/README.txt" <<'EOF'
Happy Downloader
================

  happy-downloader   the application
  yt-dlp             the downloader
  ffmpeg             for merging streams, and for putting a recording into a
                     container that plays

The application is used from a browser at

  http://localhost:3033/

It runs in the background. There is no window to keep open, and closing the
browser does not stop it; the settings page has a quit action.

Where things are
----------------

  downloads   Downloads/happy-downloader, under the user's home folder
  settings    ~/.config/happy-downloader/config.yml on Linux
              %APPDATA%\happy-downloader\config.yml on Windows
  log         the data directory, named in the settings under logging.log_path

The downloads folder, the proxy and the PO Token provider are changed on the
settings page. The rest of the configuration is edited by hand.

Cookies come from a file by default. To read them from a browser instead, so
that there is nothing to export and nothing to keep fresh:

  cookies:
    from_browser: chrome     # or edge, firefox, brave, opera, vivaldi

The PO Token provider is not included
-------------------------------------

YouTube asks some downloads for a token that proves the request came from a
real browser. The provider that produces it is a separate program that needs
Node beside it, and it is optional: without it, only the videos YouTube serves
to a guest are refused.

To add it, see https://github.com/Brainicism/bgutil-ytdlp-pot-provider and
point ytdlp.pot_base_url at it.
EOF

echo
echo "staged for the installer:"
for file in "$OUT"/*; do
  printf "  %-34s %s\n" "$(basename "$file")" "$(du -h "$file" | cut -f1)"
done
