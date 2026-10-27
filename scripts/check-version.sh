#!/bin/sh
# Check that a new tag follows the versioning rules.
#
# The rules are in VERSIONING.md: PATCH for a fix, MINOR for a new thing the
# user can see, MAJOR for something that breaks. The third number is the one
# that moves normally.
#
# What this enforces:
#
#   - the number is strictly greater than the last one
#   - the minor number goes up by one, not by several
#   - the major number does not change without a reason
#
# The check is on the tag, not on the commits, because deciding between a fix
# and a feature needs judgement that a script cannot have. What the script can
# do is catch the mistake that actually happened here: a version typed on a
# command line that nobody ever published.

set -eu

NEW=${1:-}
if [ -z "$NEW" ]; then
  echo "usage: $0 <new-tag>" >&2
  exit 1
fi

NEW=${NEW#v}

if ! echo "$NEW" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "the tag is not MAJOR.MINOR.PATCH: $NEW" >&2
  exit 1
fi

# The last version that was actually released.
#
# GitHub's releases are the truth here, not the tags: a tag that was pushed
# but never published is not a version anybody is running, and comparing
# against it would demand a jump for no reason.
#
# The releases are read from the git tags rather than from the API, so the
# check works offline and in a fork. A tag counts as released when the commit
# it points at is on the default branch — which is what publishing a release
# from a tag does.
# The new tag itself is excluded: it is in the list, and it is an ancestor of
# HEAD, and it is not what the new version is being compared against.
LAST=$(git tag --list 'v[0-9]*' --sort=-v:refname \
  | head -20 \
  | while read -r t; do
      if [ "$t" = "v$NEW" ]; then
        continue
      fi
      if git merge-base --is-ancestor "$t" HEAD 2>/dev/null; then
        echo "${t#v}"
        break
      fi
    done)

if [ -z "$LAST" ]; then
  echo "no earlier version to compare against; $NEW is the first" >&2
  exit 0
fi

echo "last released: $LAST"
echo "new release:   $NEW"

old_major=$(echo "$LAST" | cut -d. -f1)
old_minor=$(echo "$LAST" | cut -d. -f2)
old_patch=$(echo "$LAST" | cut -d. -f3)

new_major=$(echo "$NEW" | cut -d. -f1)
new_minor=$(echo "$NEW" | cut -d. -f2)
new_patch=$(echo "$NEW" | cut -d. -f3)

if [ "$new_major" -lt "$old_major" ]; then
  echo "the major number went backwards" >&2
  exit 1
fi

if [ "$new_major" -gt "$old_major" ]; then
  echo "the major number changed ($old_major -> $new_major)." >&2
  echo "That is for something that breaks an existing setup." >&2
  echo "If it is intended, say so in the release notes and re-run." >&2
  exit 1
fi

if [ "$new_minor" -lt "$old_minor" ]; then
  echo "the minor number went backwards" >&2
  exit 1
fi

if [ "$new_minor" -gt "$old_minor" ]; then
  # Going up more than one is a jump. 4.6.0 to 4.7.0 is fine; 4.6.0 to 4.9.0
  # means three versions were skipped, and that is how the numbers stop
  # meaning anything.
  skipped=$((new_minor - old_minor))

  if [ "$skipped" -gt 1 ]; then
    echo "the minor number went $old_minor -> $new_minor, skipping $skipped." >&2
    echo "Release the intermediate versions, or retag this one." >&2
    exit 1
  fi
fi

# The patch number only has to increase when the minor number did not. A
# minor release starts its own patch sequence at zero: 4.6.3 is followed by
# 4.7.0, not by 4.7.4.
if [ "$new_minor" -eq "$old_minor" ] && [ "$new_patch" -le "$old_patch" ]; then
  echo "the patch number did not increase ($old_patch -> $new_patch)" >&2
  echo "A release must be a new version." >&2
  exit 1
fi

if [ "$new_minor" -eq "$old_minor" ] && [ "$new_patch" -ne "$((old_patch + 1))" ]; then
  echo "the patch number went $old_patch -> $new_patch, skipping versions." >&2
  echo "Release them one at a time, or use the next minor." >&2
  exit 1
fi

echo "ok"
