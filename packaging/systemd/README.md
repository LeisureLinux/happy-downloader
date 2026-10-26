# systemd user units for the cookie refresher.
#
# Install:
#   mkdir -p ~/.config/systemd/user
#   cp packaging/systemd/happy-downloader-cookies.* ~/.config/systemd/user/
#   # edit the ExecStart path and --output to match your setup
#   systemctl --user daemon-reload
#   systemctl --user enable --now happy-downloader-cookies.timer
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
