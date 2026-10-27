#!/bin/sh
# Every tag is a release, and every release is a tag.
#
# The version number lived in two places once: a tag on the repository and a
# release on GitHub, updated by different commands. The binary ended up
# reporting 4.10.0 while the newest published release was 4.6.0, and nothing
# noticed for three versions. This script is the thing that notices.
#
# What it reports:
#
#   - a tag with no release, which is a version somebody can build and no
#     version anybody can download
#   - a release with no tag, which is a version that cannot be rebuilt
#
# Tags before v4.1.0 predate the tag-driven workflow — they were pushed by hand
# onto a history that had no releases at all. They were deleted on 2026-10-27,
# leaving one tag per release; the baseline stays as a guard, so a clone that
# still carries them cannot fail a build.
#
# Usage:
#
#   ./scripts/check-releases.sh                  # audit
#   ./scripts/check-releases.sh --exclude v4.7.1 # audit, ignoring a tag that is
#                                                # about to be released
#
# Needs the GitHub CLI, logged in (or GH_TOKEN set). The tags come from git, so
# a shallow clone with no tags will report nothing rather than failing.

set -eu

# The first version that was released through the workflow. Everything below
# it was a tag with nothing behind it, and has been removed.
BASELINE=4.1.0
EXCLUDE=

while [ $# -gt 0 ]; do
  case "$1" in
    --exclude)
      [ $# -ge 2 ] || { echo "--exclude needs a tag" >&2; exit 2; }
      EXCLUDE=${2#v}
      shift 2
      ;;
    -h|--help)
      sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is not installed; the release list cannot be read" >&2
  exit 2
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Versions only. Tags that are not MAJOR.MINOR.PATCH — "v3.2.0-fix", "v.2.0.9" —
# are leftovers from before the workflow and are not comparable.
normalise() {
  sed 's/^v//' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' || true
}

# Compared as numbers, not as strings: 4.10.0 is newer than 4.9.0 and sorts
# before it alphabetically.
recent() {
  awk -v b="$BASELINE" -v skip="$EXCLUDE" '
    function key(v,   a, n, i, k) {
      n = split(v, a, ".")
      k = 0
      for (i = 1; i <= 3; i++) k = k * 1000 + (a[i] + 0)
      return k
    }
    BEGIN { bk = key(b) }
    key($0) >= bk && $0 != skip { print }
  '
}

git tag -l | normalise | sort -u | recent | sort > "$tmp/tags"

if ! gh release list --limit 500 --json tagName --jq '.[].tagName' > "$tmp/raw" 2>"$tmp/err"; then
  echo "could not read the releases:" >&2
  cat "$tmp/err" >&2
  exit 2
fi
normalise < "$tmp/raw" | sort -u | recent | sort > "$tmp/releases"

tags=$(wc -l < "$tmp/tags" | tr -d ' ')
releases=$(wc -l < "$tmp/releases" | tr -d ' ')
echo "tags since v$BASELINE:     $tags"
echo "releases since v$BASELINE: $releases"

# comm rather than a loop, so the output is a diff of two sorted lists and not
# a search that can miss things.
tag_only=$(comm -23 "$tmp/tags" "$tmp/releases" | sed 's/^/  v/')
release_only=$(comm -13 "$tmp/tags" "$tmp/releases" | sed 's/^/  v/')

status=0

if [ -n "$tag_only" ]; then
  echo >&2
  echo "tags with no release:" >&2
  echo "$tag_only" >&2
  echo "Each one is a version that exists in git and nowhere else. Publish it," >&2
  echo "or delete the tag — the number cannot be both released and not." >&2
  status=1
fi

if [ -n "$release_only" ]; then
  echo >&2
  echo "releases with no tag:" >&2
  echo "$release_only" >&2
  echo "A release whose tag is gone cannot be rebuilt from the repository." >&2
  status=1
fi

if [ "$status" -eq 0 ]; then
  echo "ok: every tag since v$BASELINE has a release, and every release has a tag"
fi

exit "$status"
