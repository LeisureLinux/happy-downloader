# Happy Downloader · 欢喜下载

A self-hosted web UI for [yt-dlp](https://github.com/yt-dlp/yt-dlp). One static
binary, no runtime dependencies, no database server — point a browser at it
from anywhere on your network and download videos onto the machine that has the
disk space.

It was written for a specific awkward situation: a headless Linux box with no
browser and no desktop session, which nonetheless needs to download things that
require a logged-in session. The cookies are refreshed on another machine and
pushed over; the server just watches the file.

## What it does

**Downloads from a web page.** Paste one or more URLs, pick what you want, and
the work happens on the server. The page shows a queue, live progress, and the
full yt-dlp log.

**Site profiles.** Every site gets its own argument set, matched by hostname
(longest suffix wins, an empty host is the fallback). Editable in Settings, no
restart needed. Each invocation passes `--ignore-config`, so whatever is in the
host's own yt-dlp config cannot quietly change a download — the argument list
you see in the log is the complete one.

**Cookie files, watched.** `paths.cookie_path` accepts either a browser
extension's JSON export or a plain `cookies.txt`. The format is detected from
the content and converted with `httpOnly` preserved. The file is watched, so
replacing it refreshes the session without a restart. Each yt-dlp invocation
gets its own temporary copy, because `--cookies` also *writes back* and a shared
file lets one process truncate a file another is reading.

**PO Token support.** Point `ytdlp.pot_base_url` at a
[bgutil-ytdlp-pot-provider](https://github.com/Brainicism/bgutil-ytdlp-pot-provider)
instance and the token argument is composed for you, proxy included.

**A queue, not a history.** Only unfinished downloads are listed. A finished
file stops being a queue entry and appears in the file browser instead, which
refreshes itself when something completes.

**Argument preview.** Settings shows the exact command line a URL resolves to,
defaults and all. It is the same string the log records, so a failure can be
reproduced by copying it.

**Log panel with history.** The log button in the bottom-right corner opens a
panel that starts with the last 200 lines and then streams live over SSE. Each
request line carries the client address, so it is clear which machine on the LAN
asked for what.

**16 languages**, including Simplified and Traditional Chinese, each named in
its own script in the language menu.

**Themes.** Light and dark, plus a choice of accent colour.

## Requirements

- Linux (amd64 or arm64 for the prebuilt binaries; it builds anywhere Go does)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) on `PATH`, or an absolute path in
  the config
- `ffmpeg`, if you want merged video or audio conversion
- A JavaScript runtime (`node` or `deno`) for YouTube's challenge solving
- Optional: a PO Token provider, if YouTube asks you to prove you are not a bot

## Install

Download the binary for your architecture from
[Releases](https://github.com/LeisureLinux/happy-downloader/releases) and put it
somewhere stable.

```sh
mkdir -p ~/.local/share/happy-downloader/bin
curl -L -o ~/.local/share/happy-downloader/bin/happy-downloader \
  https://github.com/LeisureLinux/happy-downloader/releases/latest/download/happy-downloader_linux-arm64
chmod +x ~/.local/share/happy-downloader/bin/happy-downloader
```

Write a config file and start it:

```sh
~/.local/share/happy-downloader/bin/happy-downloader -conf ~/.config/happy-downloader/config.yml
```

The server listens on `0.0.0.0:3033` by default and serves the frontend from
the same binary.

### Running it as a user service

A user unit rather than a system one, because reading a browser's cookie
database needs the login keyring and that only exists inside a session bus.

`~/.config/systemd/user/happy-downloader.service`:

```ini
[Unit]
Description=Happy Downloader
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h
ExecStart=%h/.local/share/happy-downloader/bin/happy-downloader -conf %h/.config/happy-downloader/config.yml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now happy-downloader.service
loginctl enable-linger "$USER"   # keep it running after you log out
```

### Reverse proxy

The app has no TLS and no login screen of its own. Put it behind nginx and keep
it on the LAN. A ready config is at
[`packaging/nginx/happydl.conf`](packaging/nginx/happydl.conf); the parts that
matter are the WebSocket upgrade headers, `proxy_buffering off` for the log
stream, and long timeouts for large files.

Set `server.base_url` if you serve it under a subdirectory.

### Refreshing cookies from another machine

A headless host has no browser, so the cookie export has to come from somewhere
else. [`scripts/refresh-cookies.sh`](scripts/refresh-cookies.sh) reads a browser
profile, filters to the sites you care about, refuses to write a guest session,
and can push the result to the server in one step:

```sh
scripts/refresh-cookies.sh \
  --output ~/.config/happy-downloader/youtube-cookie.json \
  --browser chrome+gnomekeyring \
  --deploy user@server:/home/user/.config/happy-downloader/youtube-cookie.json
```

A matching systemd timer is in [`packaging/systemd/`](packaging/systemd/); the
notes there explain the keyring requirement.

## Configuration

`-conf` takes the path to a YAML file, default `./config.yml`.

```yaml
server:
  host: 0.0.0.0          # default; use 127.0.0.1 behind a reverse proxy
  port: 3033
  base_url: ""           # set when served under a subdirectory
  queue_size: 4          # concurrent downloads; defaults to the CPU count

paths:
  download_path: /srv/media/downloads
  downloader_path: yt-dlp          # or an absolute path
  local_database_path: /var/lib/happy-downloader
  js_runtime_path: /usr/bin/node   # used for YouTube challenge solving
  cookie_path: /etc/happy-downloader/youtube-cookie.json

ytdlp:
  pot_base_url: http://127.0.0.1:4416   # bgutil PO Token provider
  proxy: http://127.0.0.1:8888          # egress proxy, if the host needs one

logging:
  log_path: /var/log/happy-downloader.log
  enable_file_logging: false
```

Relative paths resolve against the config file's directory.

Every key can also be set through the environment with an `APP_` prefix and the
dots replaced by underscores — `APP_SERVER_PORT=8080`,
`APP_PATHS_COOKIE_PATH=/etc/cookies.json`. This keeps secrets out of the config
file without any extra machinery.

Per-site overrides — a different proxy, a different cookie file, extra
arguments — live in Settings and are stored in the local database, not here.

### Access control

There is none. The login screen, the JWT middleware and the OpenID integration
were removed: this is meant to sit on a home network, and a half-wired login —
one that is easy to leave off and hard to configure — is worse than none. Keep
it off the public internet with the reverse proxy and, if you need it, put
authentication there.

## Building

```sh
cd frontend && pnpm install && pnpm build && cd ..

CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerVersion=$(git describe --tags --always) \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerBuiltAt=$(date +%s)" \
  -o happy-downloader main.go
```

The frontend is embedded into the binary with `go:embed`, so the order matters:
build the frontend first.

Requires Go 1.26+ and Node with pnpm.

## HTTP API

The frontend talks to the backend over JSON-RPC at `POST /rpc/http` (and
`/rpc/ws` for live state). A few plain endpoints sit alongside it:

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/version` | Server and yt-dlp version |
| `GET /api/v1/cookies/status` | Cookie file state |
| `GET /api/v1/profiles` | Site profiles |
| `POST /filebrowser/downloaded` | List downloaded files |
| `GET /filebrowser/d/{id}` | Download a file |
| `GET /filebrowser/v/{id}` | Stream a file for playback |
| `GET /log/sse`, `/log/ws` | Live log stream |
| `GET /status` | Queue and connection state |

Probing a URL for its streams is the JSON-RPC method `Service.Formats`; the
frontend calls it through `/rpc/http` like everything else.

The full OpenAPI document is served at `/openapi`.

## License

GPL-3.0-only. See [LICENSE](LICENSE).

This is a fork of [marcopeocchi/yt-dlp-web-ui](https://github.com/marcopeocchi/yt-dlp-web-ui),
which is also GPL-3.0. The original copyright notice and licence are kept
intact; the modifications in this fork are released under the same terms.

yt-dlp itself is Unlicense; ffmpeg, which yt-dlp calls, has its own licensing
that you should check before redistributing anything you download.

## 中文

[README.zh_CN.md](README.zh_CN.md)
