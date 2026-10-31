# The bundled WebView2 runtime

This folder is where a WebView2 runtime goes when the installer is meant to
carry one. It is empty in the repository, and it is not committed: the files
are Microsoft's, they are over 250 MB, and they have their own licence.

A build without them still works. Windows 10 and 11 have the Evergreen
WebView2 runtime and update it themselves; the application uses it and the
window appears. Only Windows 7 needs its own copy, because the last runtime
that runs there is version 109 and it can no longer be installed.

## Putting it here

1. Go to <https://developer.microsoft.com/en-us/microsoft-edge/webview2/>
2. Download **Fixed Version** for **x64**, version **109** (the last one that
   supports Windows 7). It arrives as a `.cab`.
3. Unpack it here:

   ```
   mkdir tmp && cd tmp
   expand ../Microsoft.WebView2.FixedVersionRuntime.109.*.x64.cab -F:* .
   ```

   The result contains a folder named like
   `Microsoft.WebView2.FixedVersionRuntime.109.0.1518.xx.x64`. Move its
   **contents** into this folder, so that `msedgewebview2.exe` sits directly in
   `packaging/windows/webview2/`.
4. `scripts/stage.sh` copies the folder into the staging directory when it is
   there, the installer ships it, and the application points WebView2 at it with
   `WEBVIEW2_BROWSER_EXECUTABLE_FOLDER` — so it is used instead of, and not
   instead of only, whatever the machine has.

Do not unpack it with Explorer: it does not produce the folder structure the
runtime needs.
