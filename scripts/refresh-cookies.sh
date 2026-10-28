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
#                      [--keyring gnomekeyring] [--deploy user@host:path]
#                      [--check-only] [--dry-run]
#
# --deploy copies the export to another host after writing it, which is what a
# headless server needs: it has no browser and cannot refresh its own cookies.
# The copy is atomic on the far side — written to a temp name and renamed — so
# the watcher there never reads a half-written file.
#
# --keyring defaults to gnomekeyring, and that default is load-bearing: yt-dlp
# infers the backend from XDG_CURRENT_DESKTOP and gives up when it does not
# recognise the session, leaving every v11 cookie undecrypted. The export then
# contains LOGIN_INFO but no HSID or SID, which is a guest session wearing the
# costume of a signed-in one.
#
# It refreshes on every run by default. Detecting an expired session from
# outside is not reliable: yt-dlp does not validate a cookie file, it silently
# falls back to a guest session, so a probe against a public video succeeds
# whether the credential is good, rotated or absent. The only reliable test is
# a request that requires authentication, which means depending on a specific
# video staying gated.
#
# Reading the browser's cookies takes about fifteen seconds and the browser is
# the source of truth, so the honest design is to copy it every time and stop
# trying to guess. --check-only opts back into probing for the case where the
# browser is on another machine.
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
# yt-dlp picks a keyring backend from XDG_CURRENT_DESKTOP, which is unset or
# unknown on tiling window managers and other minimal sessions. When it cannot
# choose, it silently fails to decrypt v11 cookies — the ones holding HSID and
# SID — and produces an export that looks fine and is a guest session. Naming
# the backend explicitly is the difference between 171 cookies and 2275.
KEYRING="${COOKIE_KEYRING:-gnomekeyring}"
FORCE=1
DRY_RUN=0
DEPLOY=""
YTDLP="${YTDLP:-yt-dlp}"

log() { printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
die() { log "ERROR: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --output)    OUTPUT="${2:-}"; shift 2 ;;
    --check-url) CHECK_URL="${2:-}"; shift 2 ;;
    --browser)   BROWSER="${2:-}"; shift 2 ;;
    --keyring)   KEYRING="${2:-}"; shift 2 ;;
    --check-only) FORCE=0; shift ;;
    --deploy)     DEPLOY="${2:-}"; shift 2 ;;
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

if [ "$FORCE" -eq 0 ]; then
  if still_valid; then
    log "cookie still valid, nothing to do"
    exit 0
  fi
  log "cookie rejected, refreshing from $BROWSER"
else
  log "refreshing from $BROWSER"
fi

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
BROWSER_SPEC="$BROWSER"
[ -n "$KEYRING" ] && BROWSER_SPEC="$BROWSER+$KEYRING"

DUMP_OUT=$("$YTDLP" --cookies-from-browser "$BROWSER_SPEC" --cookies "$TMP" \
     --skip-download "$CHECK_URL" 2>&1)
DUMP_RC=$?

# A partial export is the failure mode that cost hours: it has the shape of a
# valid jar and the content of a signed-out one. yt-dlp says how many cookies it
# could not decrypt, and only mentions it when the number is not zero.
if printf '%s' "$DUMP_OUT" | grep -q "could not be decrypted"; then
  log "WARNING: $(printf '%s' "$DUMP_OUT" | grep -oE '[0-9]+ could not be decrypted')" >&2
  log "         the export may be a guest session; pass --keyring if the desktop is unusual" >&2
fi

if [ "$DUMP_RC" -ne 0 ]; then
  # The dump is written even when the request fails, so a non-zero exit here is
  # only fatal if nothing usable came out.
  if [ ! -s "$TMP" ]; then
    log "ERROR: could not read cookies from $BROWSER" >&2
    exit 1
  fi
fi

FILTERED="$WORKDIR/filtered.txt"

head -1 "$TMP" > "$FILTERED"
# The sites the application can download from. A cookie is worth keeping for
# a site that needs one: Douyin refuses a request without the browser's own
# cookies, and the browser that has them is the only source of them.
awk -F'\t' '
  /^#/ && !/^#HttpOnly_/ { next }
  NF < 7 { next }
  {
    domain = $1
    sub(/^#HttpOnly_/, "", domain)
    if (domain ~ /(^|\.)youtube\.com$/ || domain ~ /(^|\.)google\.com$/ ||
        domain ~ /(^|\.)youtu\.be$/ ||
        domain ~ /(^|\.)douyin\.com$/ || domain ~ /(^|\.)tiktok\.com$/ ||
        domain ~ /(^|\.)bilibili\.com$/ || domain ~ /(^|\.)xiaohongshu\.com$/ ||
        domain ~ /(^|\.)weibo\.com$/ || domain ~ /(^|\.)instagram\.com$/ ||
        domain ~ /(^|\.)reddit\.com$/ || domain ~ /(^|\.)vimeo\.com$/)
      print
  }
' "$TMP" >> "$FILTERED"

COUNT=$(grep -vc '^#' "$FILTERED" || true)
if [ "${COUNT:-0}" -eq 0 ]; then
  log "ERROR: no youtube cookies found; is the browser signed in?" >&2
  exit 1
fi

if ! grep -q LOGIN_INFO "$FILTERED" && ! grep -qi douyin "$FILTERED"; then
  # The browser is signed out, or the cookie has not been written yet. Writing
  # this would replace a working credential with a guest session, which
  # downloads public videos and fails everything else — a strictly worse state
  # that is also harder to notice, because the file still looks present.
  if [ -f "$OUTPUT" ] && grep -q LOGIN_INFO "$OUTPUT"; then
    log "browser has no LOGIN_INFO (signed out?); keeping the existing export" >&2
    exit 1
  fi
  log "WARNING: no LOGIN_INFO in the export; the browser may not be signed in"
fi

if [ "$DRY_RUN" -eq 1 ]; then
  log "dry run: would write $COUNT cookies to $OUTPUT"
  exit 0
fi

mkdir -p "$(dirname "$OUTPUT")"
install -m 600 "$FILTERED" "$OUTPUT"
log "wrote $COUNT cookies to $OUTPUT"

# Structural check only. A network probe cannot tell a good session from a
# bad one — a public video answers either way — so requesting one here would
# cost time and prove nothing. What can be checked is that the file is a
# well-formed Netscape jar with the cookie that decides sign-in.
if [ "$(grep -vc '^#' "$OUTPUT" || true)" -lt 5 ]; then
  log "ERROR: the written file has suspiciously few cookies" >&2
  exit 1
fi

if [ -n "$DEPLOY" ]; then
  host="${DEPLOY%%:*}"
  dest="${DEPLOY#*:}"
  if [ "$host" = "$dest" ]; then
    log "ERROR: --deploy wants user@host:/path" >&2
    exit 1
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    log "dry run: would copy to $DEPLOY"
    exit 0
  fi

  # Write to a temporary name and rename, so the watcher on the far side never
  # sees a partial file and converts half a jar.
  if scp -q -o BatchMode=yes "$OUTPUT" "$host:$dest.tmp" &&
     ssh -o BatchMode=yes "$host" "mv '$dest.tmp' '$dest' && chmod 600 '$dest'"; then
    log "copied to $DEPLOY"
  else
    log "ERROR: could not copy to $DEPLOY" >&2
    exit 1
  fi
fi

log "done"
exit 0
