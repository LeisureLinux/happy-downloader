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
- A JavaScript runtime (`node` 20+) — YouTube's challenge solving fails without
  it, and the failure looks like missing formats rather than a missing runtime
- A PO Token provider — YouTube now requires one for most content; see
  [Installing for an AI agent](#installing-for-an-ai-agent) for the setup

Requirements are the part people underestimate. If you are installing this for
someone, read that section instead of this list: it covers the four external
components in the order they have to be set up, and the checks that tell you
each one works before moving on.

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

## Installing for an AI agent

> **If you are an agent installing this on a user's machine, read this whole
> section before running anything.** It is written to be followed directly:
> every command is complete, every version is one that was tested together, and
> the order matters. Happy Downloader is the smallest part — it is a single
> static binary. The work is the four things it shells out to.

### What has to be present

| Component | Why | Required |
|---|---|---|
| `yt-dlp` | Does the actual downloading | Yes |
| A JavaScript runtime (`node`) | Solves YouTube's challenge (`n` parameter); without it YouTube returns images only, or nothing | Yes, for YouTube |
| `ffmpeg` | Merges separate video and audio streams; converts audio formats | Yes, unless every download is a single stream |
| A PO Token provider | Proves the session is not a bot. YouTube now requires it for most content; without it downloads fail with "Sign in to confirm you're not a bot" | Yes, for YouTube |
| A cookie export | Carries the logged-in session. Required for age-gated, members-only or region-locked content, and increasingly for ordinary videos | Practically yes |
| Happy Downloader | The web UI | Yes |

`yt-dlp` ships a release roughly monthly and YouTube breaks older versions
regularly. Install it where it can be updated without rebuilding this project —
the binary reads the path from config and does not bundle it.

### Step 1 — system packages

Debian/Ubuntu. Substitute the equivalent for the distribution in front of you;
`node` must be 20 or newer, `ffmpeg` any recent build.

```sh
sudo apt update
sudo apt install -y ffmpeg python3 python3-venv curl
# Node 20+ — the distribution's nodejs may be too old
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
node --version    # must print v20 or newer
```

### Step 2 — yt-dlp in its own virtualenv

Not `apt install yt-dlp` and not `pip install --user`: a distribution package
goes stale for months, and a system-wide pip install conflicts with the
distribution's own Python packages. A venv is disposable and updatable.

```sh
python3 -m venv ~/.local/share/happy-downloader/venv
~/.local/share/happy-downloader/venv/bin/pip install -U pip
~/.local/share/happy-downloader/venv/bin/pip install -U yt-dlp
~/.local/share/happy-downloader/venv/bin/yt-dlp --version
```

Updating later is `pip install -U yt-dlp` in the same venv. When downloads
start failing across the board, do that first — it fixes most of them.

### Step 3 — the PO Token provider

This is the step most installations get wrong, and the one whose absence
produces the most confusing error. `bgutil-ytdlp-pot-provider` has two halves:
a **yt-dlp plugin** (installed into the venv, so yt-dlp knows how to ask) and a
**server** (a Node process that actually generates the token).

```sh
# the plugin, into the same venv as yt-dlp
~/.local/share/happy-downloader/venv/bin/pip install -U bgutil-ytdlp-pot-provider

# the server
cd ~/.local/share/happy-downloader
git clone --depth 1 https://github.com/Brainicism/bgutil-ytdlp-pot-provider.git
cd bgutil-ytdlp-pot-provider/server
npm install
npx tsc          # produces build/main.js
```

Check it works before wiring it up. It takes a few seconds per token:

```sh
node build/main.js --host 127.0.0.1 --port 4416 &
curl -s -X POST http://127.0.0.1:4416/get_pot \
  -H 'Content-Type: application/json' \
  -d '{"content_binding":"dQw4w9WgXcQ"}' | head -c 200
```

A JSON object with a `poToken` field means it works. An empty response or a
timeout means it cannot reach YouTube — see the proxy note below.

Run it as a service so it survives reboots. `~/.config/systemd/user/bgutil-pot.service`:

```ini
[Unit]
Description=bgutil PO Token provider
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h/.local/share/happy-downloader/bgutil-ytdlp-pot-provider/server
ExecStart=/usr/bin/node build/main.js --host 127.0.0.1 --port 4416
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now bgutil-pot.service
```

### Step 4 — the cookie export

The server has no browser. It cannot log in, and it cannot extract cookies from
a profile. The export has to be produced on a machine that has one and copied
over.

**Produce it on the machine with the browser.** Either use a cookie-export
extension (the "Get cookies.txt LOCALLY" family — check that it is the local
variant, which does not upload anything) and save as JSON or Netscape format,
or use the script in this repository, which reads the profile directly:

```sh
scripts/refresh-cookies.sh --output ~/youtube-cookie.json --browser chrome
```

The script needs the login keyring, so it only works inside a desktop session.
That is why it is a *user* unit and not a system one.

**Copy it to the server** and point `paths.cookie_path` at it:

```sh
scp ~/youtube-cookie.json user@server:/home/user/.config/happy-downloader/youtube-cookie.json
```

The server watches that file. Replacing it refreshes the session with no
restart — which is why the export is pushed rather than configured once.

Two failure modes worth knowing:

- **A guest export still contains `LOGIN_INFO`**, so "the file has cookies" is
  not evidence that it has a *session*. Verify with
  `~/.local/share/happy-downloader/venv/bin/yt-dlp --cookies <file> --print title <url>`
  and check that the title resolves.
- **`chrome` alone may silently fail to decrypt v11 cookies** when yt-dlp cannot
  identify the keyring backend. Pass `chrome+gnomekeyring` (or `+kwallet`)
  explicitly. The export will still look plausible — it will contain roughly
  170 cookies instead of a few thousand.

### Step 5 — Happy Downloader itself

```sh
mkdir -p ~/.local/share/happy-downloader/bin ~/.config/happy-downloader
curl -L -o ~/.local/share/happy-downloader/bin/happy-downloader \
  https://github.com/LeisureLinux/happy-downloader/releases/latest/download/happy-downloader_linux-amd64
chmod +x ~/.local/share/happy-downloader/bin/happy-downloader
```

Use `linux-arm64` on a Raspberry Pi or other ARM host. Then write
`~/.config/happy-downloader/config.yml` — every path below is absolute, because
a user service starts in the home directory and relative paths would land
somewhere unexpected:

```yaml
server:
  host: 127.0.0.1
  port: 3033

paths:
  download_path: /home/USER/media/downloads
  downloader_path: /home/USER/.local/share/happy-downloader/venv/bin/yt-dlp
  local_database_path: /home/USER/.local/share/happy-downloader/data
  js_runtime_path: /usr/bin/node
  cookie_path: /home/USER/.config/happy-downloader/youtube-cookie.json

ytdlp:
  pot_base_url: http://127.0.0.1:4416
  # proxy: http://127.0.0.1:8888   # only if the host needs one to reach YouTube
```

Install the user unit from [Running it as a user service](#running-it-as-a-user-service)
and start it. Note `loginctl enable-linger` — without it the service stops when
the last session closes, which looks like a crash.

### Step 6 — verify

The useful test is the whole line at once, because that is what the server runs:

```sh
~/.local/share/happy-downloader/venv/bin/yt-dlp \
  --cookies ~/.config/happy-downloader/youtube-cookie.json \
  --js-runtimes node:/usr/bin/node \
  --extractor-args 'youtubepot-bgutilhttp:base_url=http://127.0.0.1:4416' \
  --remote-components ejs:github \
  --print '%(title)s | %(ext)s' 'https://youtu.be/dQw4w9WgXcQ'
```

A title and a format means the four components are wired together correctly.

**Do not shorten it to test one part at a time.** yt-dlp without a PO Token
provider now fails on most YouTube videos with `Only images are available` or
`Requested format is not available` — an error that names none of the missing
pieces. A partial command therefore fails on a working installation, which
sends you looking for a fault that is not there. The all-at-once line only
passes when everything is in place, which is exactly the question being asked.

Then confirm the server sees the same thing:

```sh
# the server is up and can find yt-dlp
curl -s http://127.0.0.1:3033/api/v1/version

# end to end, through the app
curl -s -X POST http://127.0.0.1:3033/rpc/http \
  -H 'Content-Type: application/json' \
  -d '{"method":"Service.Exec","params":[{"url":"https://youtu.be/dQw4w9WgXcQ","params":[]}],"id":1}'
```

The last call returns an id. Open the web UI, press the log button in the top
bar, and check that the `msg="yt-dlp exec"` line carries the full argument list.
That line is the ground truth for everything the server does — if a download
behaves differently from the command above, the difference is visible there.

### When it does not work

The failure messages are specific and each maps to one cause:

| Message | Cause |
|---|---|
| `Sign in to confirm you're not a bot` | No cookie file, or a guest export, or an expired session. Re-export. |
| `Requested format is not available` / `Only images are available` | No PO Token provider, or the JavaScript runtime is missing. Check steps 2 and 3. |
| `n challenge solving failed` | The JS runtime is missing or too old. `node --version` must be 20+. |
| `HTTP Error 429` on subtitles | YouTube rate-limits captions. The server passes `--ignore-errors` so the video still downloads; this is expected. |
| Everything times out | The host cannot reach YouTube. Check whether it needs a proxy and set `ytdlp.proxy`. |
| The service stops when you log out | `loginctl enable-linger` was not run. |

Every one of these is visible in the log panel with the full command line, which
is what makes them diagnosable rather than mysterious.

### If the host needs a proxy

On a network where YouTube is unreachable directly — a poisoned DNS resolver, a
blocked range — set `ytdlp.proxy` and the server passes `--proxy` on every
invocation. The PO Token provider needs the same egress but has no `--proxy`
flag of its own; the server carries the proxy into its extractor-args value, so
setting it once is enough for both.

Check the host before assuming: if `curl -sI https://www.youtube.com` returns
`000` or a reset, direct access is not available and a proxy is not optional.

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
./scripts/build.sh
```

That is the whole thing. It builds the frontend and then the binary, and it
removes `frontend/dist` first — the frontend is embedded with `go:embed`, so a
stale bundle would otherwise be compiled in silently and the binary would
carry the previous version's interface without any error. Removing it turns
that into a build failure.

Requires Go 1.26+ and Node with pnpm.

To build for another architecture, set `GOARCH` and `GOOS`:

```sh
GOARCH=arm64 GOOS=linux ./scripts/build.sh happy-downloader_linux-arm64
```

Releases are built by GitHub Actions rather than by hand: push a tag and the
workflow compiles both architectures, checksums them and publishes. See
[`.github/workflows/release.yml`](.github/workflows/release.yml).

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


## License

GPL-3.0-only. See [LICENSE](LICENSE).

This is a fork of [marcopeocchi/yt-dlp-web-ui](https://github.com/marcopeocchi/yt-dlp-web-ui),
which is also GPL-3.0. The original copyright notice and licence are kept
intact; the modifications in this fork are released under the same terms.

yt-dlp itself is Unlicense; ffmpeg, which yt-dlp calls, has its own licensing
that you should check before redistributing anything you download.

## 中文

[README.zh_CN.md](README.zh_CN.md)
