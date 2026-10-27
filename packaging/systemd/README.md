# systemd user units for the cookie refresher.
#
# Install:
#   scripts/install-cookie-timer.sh \
#       --deploy user@server:/home/user/.config/happy-downloader/youtube-cookie.json
#
# The script fills in this checkout's absolute path and enables the timer. Do
# not copy the units by hand and edit the path: a user unit has no working
# directory, so the path is a copy of where the repository happens to live, and
# it goes stale — silently — the moment the directory is renamed. The timer
# keeps firing, the service fails with 203/EXEC, and the only visible symptom is
# that the cookie file's timestamp stops moving. Re-run the script after moving
# the checkout; that is the whole repair.
#
# Why a user unit rather than a system one: reading the browser's cookie
# database needs the login keyring, which only exists inside the user's session.
# A system service has no keyring and cannot decrypt the cookies.
#
# Why a timer rather than cron: cron jobs run without DBUS_SESSION_BUS_ADDRESS,
# so they cannot reach the keyring either. A user timer runs in the session and
# inherits it.
#
# Check on demand:
#   systemctl --user start happy-downloader-cookies.service
#   journalctl --user -u happy-downloader-cookies.service -n 30

## Getting the export to a headless host

The refresher needs a browser, so it runs on the machine that has one. A
headless host such as an Orange Pi has no Chrome and no keyring, which means
`--cookies-from-browser` cannot work there — copying the file over is the only
option, and it is the workflow the cookie watcher was built for: replace the
file and the server picks it up within a second, no restart.

Add the copy to the service, after the refresh:

```ini
ExecStartPost=/usr/bin/scp -q %h/.config/happy-downloader/youtube-cookie.json \
    pi@orangepi:/data/happy-downloader/youtube-cookie.json
```

Set up an SSH key for that first, or the timer will block on a password prompt.

Alternatively, point `--output` at a directory the Pi mounts over NFS or SMB.

## Why it refreshes every run instead of checking first

Detecting an expired session from outside is not reliable. yt-dlp does not
validate a cookie file — it silently falls back to a guest session — so a probe
against a public video answers the same whether the credential is good, rotated
or missing entirely. The only dependable test is a request that requires
authentication, which means depending on a specific video staying gated.

Reading the browser's cookies takes about fifteen seconds and the browser is
the source of truth, so the script copies it on every run and stops guessing.
`--check-only` opts back into probing, for the case where the browser lives on
another machine.

## Testing without waiting for the timer

```bash
systemctl --user start happy-downloader-cookies.service
journalctl --user -u happy-downloader-cookies.service -n 30 --no-pager
```

`--force` re-exports even when the current file still works; `--dry-run` shows
what would be written without touching anything:

```bash
./scripts/refresh-cookies.sh --output /tmp/x.json --force --dry-run
```
