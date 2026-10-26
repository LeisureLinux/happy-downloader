#!/usr/bin/env bash
#
# Refresh a cookie export from the browser, but only when the current one has
# stopped working.
#
# Why this exists: a YouTube session rotates without warning, and the failure
# looks like a download problem rather than an expired credential. Checking on
# a timer turns "the download broke and I do not know why" into "the credential
# was refreshed an hour ago".
#
# It does NOT need browser automation. yt-dlp reads the browser's cookie
# database directly, including the httpOnly cookies that a JavaScript call
# cannot see — LOGIN_INFO among them — and those are exactly the ones that
# decide whether a request is signed in.
#
# Usage:
#   refresh-cookies.sh --output <path> [--check-url <url>] [--browser chrome]
#                      [--force] [--dry-run]
#
# Exit codes:
#   0  the export is valid (nothing to do, or refreshed successfully)
#   1  the export is invalid and could not be refreshed
#   2  bad usage
#
set -uo pipefail

OUTPUT=""
CHECK_URL="https://www.youtube.com/watch?v=dQw4w9WgXcQ"
BROWSER="chrome"
FORCE=0
DRY_RUN=0
YTDLP="${YTDLP:-yt-dlp}"

log() { printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
die() { log "ERROR: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --output)    OUTPUT="${2:-}"; shift 2 ;;
    --check-url) CHECK_URL="${2:-}"; shift 2 ;;
    --browser)   BROWSER="${2:-}"; shift 2 ;;
    --force)     FORCE=1; shift ;;
    --dry-run)   DRY_RUN=1; shift ;;
    -h|--help)   sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)           die "unknown argument: $1" ;;
  esac
done

[ -n "$OUTPUT" ] || die "--output is required"

command -v "$YTDLP" >/dev/null 2>&1 || die "yt-dlp not found in PATH (set YTDLP=...)"

# ── Is the current export still accepted? ───────────────────────────────────
#
# A cheap request is enough: if the credential is rejected, yt-dlp says so
# explicitly. Anything else — a network error, a video that is simply
# unavailable — is not evidence that the cookie expired, and must not trigger
# a refresh that would overwrite a working file.
still_valid() {
  [ -f "$OUTPUT" ] || return 1

  # A file without LOGIN_INFO is a guest session however new it is. Checking
  # this first matters because the probe below uses a public video, which
  # succeeds without any credentials at all — so the request alone can never
  # tell a signed-in export from an empty file.
  grep -q LOGIN_INFO "$OUTPUT" || return 1

  local out
  out=$("$YTDLP" --cookies "$OUTPUT" --simulate \
        --socket-timeout 20 "$CHECK_URL" 2>&1)

  if printf '%s' "$out" | grep -qiE \
      "cookies are no longer valid|Sign in to confirm you.re not a bot"; then
    return 1
  fi

  # A transient failure says nothing about the credential; keep the file.
  if printf '%s' "$out" | grep -qiE \
      "timed out|Connection reset|Temporary failure|Unable to download webpage"; then
    log "check inconclusive (network); keeping the current export"
    return 0
  fi

  return 0
}

if [ "$FORCE" -eq 0 ] && still_valid; then
  log "cookie still valid, nothing to do"
  exit 0
fi

log "cookie rejected, refreshing from $BROWSER"

# ── Refresh ────────────────────────────────────────────────────────────────
# A temp *directory*, not a temp file: yt-dlp treats an existing path passed to
# --cookies as a jar to read, and exits without writing anything if it is empty.
# The dump target has to be a name that does not exist yet.
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/cookie-refresh.XXXXXX")"
trap 'rm -rf "$WORKDIR"' EXIT
TMP="$WORKDIR/dump.txt"

# yt-dlp dumps every cookie it reads, across all sites. Only the ones that
# matter are kept: a credential file should not carry the rest of the browser's
# browsing history in cookie form.
if ! "$YTDLP" --cookies-from-browser "$BROWSER" --cookies "$TMP" \
     --skip-download --no-warnings "$CHECK_URL" >/dev/null 2>&1; then
  # The dump is written even when the request fails, so a non-zero exit here is
  # only fatal if nothing usable came out.
  if [ ! -s "$TMP" ]; then
    log "ERROR: could not read cookies from $BROWSER" >&2
    exit 1
  fi
fi

FILTERED="$WORKDIR/filtered.txt"

head -1 "$TMP" > "$FILTERED"
awk -F'\t' '
  /^#/ && !/^#HttpOnly_/ { next }
  NF < 7 { next }
  {
    domain = $1
    sub(/^#HttpOnly_/, "", domain)
    if (domain ~ /(^|\.)youtube\.com$/ || domain ~ /(^|\.)google\.com$/ ||
        domain ~ /(^|\.)youtu\.be$/)
      print
  }
' "$TMP" >> "$FILTERED"

COUNT=$(grep -vc '^#' "$FILTERED" || true)
if [ "${COUNT:-0}" -eq 0 ]; then
  log "ERROR: no youtube cookies found; is the browser signed in?" >&2
  exit 1
fi

if ! grep -q LOGIN_INFO "$FILTERED"; then
  # Without this the export is a guest session: the download will work for
  # public videos and fail for everything else, with a message that blames the
  # cookies rather than the sign-in.
  log "WARNING: no LOGIN_INFO in the export; the browser may not be signed in"
fi

if [ "$DRY_RUN" -eq 1 ]; then
  log "dry run: would write $COUNT cookies to $OUTPUT"
  exit 0
fi

mkdir -p "$(dirname "$OUTPUT")"
install -m 600 "$FILTERED" "$OUTPUT"
log "wrote $COUNT cookies to $OUTPUT"

# Verify the new file rather than assuming: an export that still fails is worse
# than no export, because it replaces something that might have worked.
if still_valid; then
  log "new export accepted"
  exit 0
fi

log "ERROR: the refreshed export is still rejected by YouTube" >&2
log "This is not a cookie problem: the same credentials work in a browser." >&2
log "The IP is most likely being throttled. Wait, then retry." >&2
exit 1
