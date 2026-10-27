# Versioning

The version is `MAJOR.MINOR.PATCH`, and the third number is what moves.

- **PATCH** — a fix. Something was broken and now works, or a message is
  clearer. This is the common case: most releases are here.
- **MINOR** — a new thing the user can see: a new column, a new panel, a new
  setting. The old ways still work.
- **MAJOR** — something that breaks: a removed feature, a changed default, a
  renamed configuration key, a database format that older builds cannot read.

So a release that adds a caption panel and fixes a button is `4.7.0`, not
`4.8.0` and not `4.6.1`. A release that only fixes two bugs is `4.6.1`.

## Why the third number matters

The version is the only thing that tells a user whether they are running
something that was tested against their setup. Skipping numbers — 4.6.0, then
4.7.0, then 4.10.0 — makes the history unreadable and makes it impossible to
tell a typo from a jump.

It also happened here: the deployed binary reported `4.10.0` while the
newest published release was `4.6.0`. The number was typed on a command line
rather than taken from a tag, and nothing noticed. The build now reads the
version from the tag and refuses to be told otherwise, so a build can only ever
report a version that was published.

## Who decides

A PATCH does not need to be approved. It is a fix, it is what most releases
are, and asking about every one of them wastes the time of the person being
asked.

A MINOR and a MAJOR are asked for first. They are announcements: a new thing
the user can see, or a change that breaks an existing setup. The number is the
cheap part of that decision — what ships, and what the release notes say, is
the expensive part.

So: patch, go ahead. Minor or major, ask.

## How a release is built

By default a release is built on the machine that is deploying it and
published from there. That is the normal case, it is a PATCH, and it does not
wait for a runner:

```sh
./scripts/build.sh                                  # must be a clean tree
git tag v4.7.1
GOOS=linux GOARCH=amd64 ./scripts/build.sh happy-downloader_linux-amd64
GOOS=linux GOARCH=arm64 ./scripts/build.sh happy-downloader_linux-arm64
sha256sum happy-downloader_linux-* > SHA256SUMS
git push origin v4.7.1
gh release create v4.7.1 \
  happy-downloader_linux-amd64 happy-downloader_linux-arm64 SHA256SUMS
```

GitHub Actions is for the releases that have to be reproducible by someone
else, or that are being announced: every MINOR, every MAJOR, and any release
asked for explicitly. That is what "发版" means — the workflow builds both
architectures and publishes them, and nothing is uploaded from a laptop:

```sh
gh workflow run release.yml -f tag=v4.8.0
```

The tag is cut first either way, and the version in the binary comes from the
tag, so a build can only report a version that exists.

## One set of numbers

A tag and a release are the same thing seen twice: the tag is what can be
built, the release is what can be downloaded. Having one without the other is
how the numbers drift apart — a tag nobody released, or a release nobody can
rebuild.

`scripts/check-releases.sh` compares the two lists and fails when they
disagree. It runs on every push to master and after every publish. Tags before
`v4.1.0` predate the workflow and are exempt; they are history, not drift.

There is also exactly one copy of the number. `frontend/package.json` has no
version field: it would only be a second place to forget to change, and
nothing reads it — the interface shows whatever the server reports, and the
server reads the tag. If a file starts needing the version at build time, take
it from `git describe` the way `scripts/build.sh` does, rather than writing it
down.

Rebuilding a release that already exists needs `replace: true`, because the
assets of an existing release are replaced rather than merged:

```sh
gh workflow run release.yml -f tag=v4.7.1 -f replace=true
```

## Choosing the number

Look at the commits since the last tag:

- Only fixes → bump PATCH.
- A new feature the user can see → bump MINOR.
- Anything removed, renamed, or changed in a way that breaks an existing
  setup → bump MAJOR.

If you are unsure between PATCH and MINOR, ask what a user would have to do
differently. A new button is a MINOR. A button that moved is a PATCH.
