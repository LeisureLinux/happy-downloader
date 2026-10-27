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

## Releasing

```sh
git tag v4.6.1
git push origin v4.6.1
```

The workflow builds both architectures, checksums them and publishes. The
version in the binary comes from the tag, so it cannot disagree with the
release.

Pushing a tag that already exists does not trigger the workflow — GitHub fires
on tag creation. To rebuild a release, dispatch the workflow by hand with the
tag and `replace: true`; without that flag it refuses, because the assets of an
existing release are replaced rather than merged.

## Choosing the number

Look at the commits since the last tag:

- Only fixes → bump PATCH.
- A new feature the user can see → bump MINOR.
- Anything removed, renamed, or changed in a way that breaks an existing
  setup → bump MAJOR.

If you are unsure between PATCH and MINOR, ask what a user would have to do
differently. A new button is a MINOR. A button that moved is a PATCH.
