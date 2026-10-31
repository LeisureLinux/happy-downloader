# The bundled WebView2 runtime

This folder is where a WebView2 runtime goes when the installer is meant to
carry one. It is empty in the repository, and it is not committed: the files are
Microsoft's, they are 197 MB compressed and 445 MB unpacked, and they have their
own licence.

The release build does not use this folder. It downloads the runtime itself,
from a pinned URL, and checks it against a pinned checksum — see
`HD_WEBVIEW2_URL` in `.github/workflows/release.yml`. This folder is for a build
made by hand.

## Why version 109

Windows 10 and 11 have the Evergreen WebView2 runtime and update it themselves,
so the application uses theirs and the folder is not needed.

Windows 7 has none, and can no longer install one: **109 is the last version
that runs there**, and Microsoft's download page keeps only the two most recent
versions — 109 was taken down. Microsoft's own documentation says to keep an
archived copy for exactly this reason.

## Getting it

`109.0.1518.78` is the last build archived in a fixed-version form:

```
https://github.com/westinyang/WebView2RuntimeArchive/releases/download/109.0.1518.78/Microsoft.WebView2.FixedVersionRuntime.109.0.1518.78.x64.cab
```

```
sha256 7622281cf83de1a35e3a471f432f7a897d65f0a7d3975df08512b7b253dd45c7
size   207090243 bytes
```

Later 109 patches — `.95`, `.115`, `.140` — exist only as Windows Update packages
in the [Microsoft Update Catalog](https://www.catalog.update.microsoft.com/Search.aspx?q=webview2%20runtime%20x64%20109),
which are installers rather than a folder that can be shipped. `.140` carries
the last security fixes for the 109 branch; `.78` is what a portable copy exists
for.

## Unpacking it here

```
mkdir -p tmp && cd tmp
7z x ../Microsoft.WebView2.FixedVersionRuntime.109.0.1518.78.x64.cab
mv Microsoft.WebView2.FixedVersionRuntime.109.0.1518.78.x64/* ../webview2/
```

So that `msedgewebview2.exe` sits directly in this folder. Then
`scripts/stage.sh` copies it into the staging directory, the installer ships it,
and the application points WebView2 at it with
`WEBVIEW2_BROWSER_EXECUTABLE_FOLDER` — used in place of, not in addition to,
whatever the machine has.

Do not unpack it with Explorer: it does not produce the folder structure the
runtime needs.
