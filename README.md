# Happy Downloader

A web UI for [yt-dlp](https://github.com/yt-dlp/yt-dlp) built to run on a
headless host: point it at a cookie export, let it watch the file, and add
sites from the browser without restarting anything.

Forked from
[LeisureLinux/happy-downloader](https://github.com/LeisureLinux/happy-downloader)
at v4.0.0. Upstream is archived; this fork carries the changes below.

## What this fork adds

- **Site profiles.** Arguments are composed from an editable table keyed by
  hostname, and every invocation passes `--ignore-config`, so nothing from the
  host's yt-dlp config affects a download. Adding a site is a new row.
- **Cookies from a path.** `paths.cookie_path` accepts a browser extension's
  JSON export or an existing cookies.txt; the format is detected and converted
  with `httpOnly` preserved. The file is watched, so replacing it refreshes the
  session with no restart.
- **Argument preview.** Settings shows the exact command line a URL resolves
  to, defaults included.
- **Version reporting.** The header carries the version and build timestamp
  injected at build time.

## Install

Static binaries, no runtime dependencies. yt-dlp, ffmpeg and a JS runtime are
still needed on the host.

```sh
docker pull ghcr.io/leisurelinux/happy-downloader:v4
```

## Configuration

```yaml
server:
  host: 0.0.0.0
  port: 3033

paths:
  download_path: /data/downloads
  downloader_path: /usr/bin/yt-dlp
  local_database_path: /data/state
  js_runtime_path: /usr/bin/node
  # Optional: a cookie export to convert and watch. JSON or Netscape.
  cookie_path: /data/youtube-cookie.json

ytdlp:
  # Optional. Per-site overrides live in the UI.
  pot_base_url: http://127.0.0.1:4416
  proxy: http://127.0.0.1:7890
```

A not so terrible web ui for yt-dlp. 

High performance extendeable web ui and RPC server for yt-dlp with low impact on resources.

Created for the only purpose of *fetching* videos from my server/nas and monitor upcoming livestreams. 

**Docker images are available on [Docker Hub](https://hub.docker.com/r/leisurelinux/happy-downloader) or [ghcr.io](https://github.com/LeisureLinux/happy-downloader/pkgs/container/yt-dlp-web-ui)**.

```sh
docker pull leisurelinux/happy-downloader:v4
```
```sh
# latest stable
docker pull ghcr.io/LeisureLinux/happy-downloader:v4
```

## Donate to happy-downloader development
[PayPal](https://paypal.me/marcofw)

*Keeps the project alive!* 😃

## Community stuff
Feel free to join :)

[Discord](https://discord.gg/GZAX5FfGzE)

## Some screeshots
![image](https://github.com/user-attachments/assets/fc43a3fb-ecf9-449d-b5cb-5d5635020c00)
![image](https://github.com/user-attachments/assets/3210f6ac-0dd8-403c-b839-3c24ff7d7d00)
![image](https://github.com/user-attachments/assets/16450a40-cda6-4c8b-9d20-8ec36282f6ed)

## Video showcase
[app.webm](https://github.com/LeisureLinux/happy-downloader/assets/35533749/91545bc4-233d-4dde-8504-27422cb26964)

## Settings

The currently avaible settings are:
-   Server address
-   Switch theme
-   Extract audio
-   Switch language
-   Optional format selection
-   Override the output filename
-   Override the output path
-   Pass custom yt-dlp arguments safely
-   Download queue (limit concurrent downloads)

## Format selection

This feature is disabled by default as this intended to be used to retrieve the best quality automatically.

To enable it just go to the settings page and enable the **Enable video/audio formats selection** flag!

## [Docker](https://github.com/LeisureLinux/happy-downloader/pkgs/container/yt-dlp-web-ui) run
```sh
docker pull leisurelinux/happy-downloader
docker run -d -p 3033:3033 -v <your dir>:/downloads leisurelinux/happy-downloader
```

Or with docker but building the container manually.

```sh
docker build -t happy-downloader .
docker run -d -p 3033:3033 -v <your dir>:/downloads happy-downloader

docker run -d -p 3033:3033 \
  -v <your dir>:/downloads \  
  -v <your dir>:/config \ # optional
  happy-downloader

```

If you opt to add RPC authentication...
```sh
docker run -d \
    -p 3033:3033 \
    -e JWT_SECRET randomsecret
    -v /path/to/downloads:/downloads \
    -v /path/for/config:/config \ # optional
    leisurelinux/happy-downloader \
    --auth \
    --user your_username \
    --pass your_pass
```

If you wish for limiting the download queue size...

e.g. limiting max 2 concurrent download.
```sh
docker run -d \
    -p 3033:3033 \
    -e JWT_SECRET randomsecret
    -v /path/to/downloads:/downloads \
    leisurelinux/happy-downloader \
    --qs 2
```

### Docker Compose
```yaml
services:
  happy-downloader:
    image: leisurelinux/happy-downloader
    ports:
      - 3033:3033
    volumes:
      - <your dir>:/downloads # replace <your dir> with a directory on your host system
    healthcheck:
      test: curl -f http://localhost:3033 || exit 1
    restart: unless-stopped
```

### ⚡ One-Click Deploy

| Cloud Provider | Deploy Button |
|----------------|---------------|
| AWS | <a href="https://deploystack.io/deploy/LeisureLinux-happy-downloader?provider=aws&language=cfn"><img src="https://raw.githubusercontent.com/deploystackio/deploy-templates/refs/heads/main/.assets/img/aws.svg" height="38"></a> |
| DigitalOcean | <a href="https://deploystack.io/deploy/LeisureLinux-happy-downloader?provider=do&language=dop"><img src="https://raw.githubusercontent.com/deploystackio/deploy-templates/refs/heads/main/.assets/img/do.svg" height="38"></a> |
| Render | <a href="https://deploystack.io/deploy/LeisureLinux-happy-downloader?provider=rnd&language=rnd"><img src="https://raw.githubusercontent.com/deploystackio/deploy-templates/refs/heads/main/.assets/img/rnd.svg" height="38"></a> |

<sub>Generated by <a href="https://deploystack.io/c/LeisureLinux-happy-downloader" target="_blank">DeployStack.io</a></sub>

## [Prebuilt binaries](https://github.com/LeisureLinux/happy-downloader/releases) installation

```sh
# download the latest release from the releases page
mv happy-downloader_linux-[your_system_arch] /usr/local/bin/happy-downloader

# /home/user/downloads as an example and yt-dlp in $PATH
happy-downloader --out /home/user/downloads

# specifying yt-dlp path
happy-downloader --out /home/user/downloads --driver /opt/soemdir/yt-dlp

# specifying using a config file
happy-downloader --conf /home/user/.config/happy-downloader.conf
```

### Arguments
```sh
Usage happy-downloader:
  -auth
        Enable RPC authentication
  -conf string
        Config file path (default "./config.yml")
  -db string
        local database path (default "local.db")
  -driver string
        yt-dlp executable path (default "yt-dlp")
  -fl
        enable file based logging
  -host string
        Host where server will listen at (default "0.0.0.0")
  -lf string
        set log file location (default "happy-downloader.log")
  -out string
        Where files will be saved (default ".")
  -pass string
        Password required for auth
  -port int
        Port where server will listen at (default 3033)
  -qs int
        Queue size (concurrent downloads) (default 2)
  -session string
        session file path (default ".")
  -user string
        Username required for auth
  -web string
        frontend web resources path
```

### Config file
By running `happy-downloader` in standalone mode you have the ability to also specify a config file.
The config file **will overwrite what have been passed as cli argument**.
With Docker, inside the mounted `/conf` volume inside there must be a file named `config.yml`.

```yaml
# Simple configuration file for yt-dlp webui

---
# Host where server will listen at (default: "0.0.0.0")
#host: 0.0.0.0

# Port where server will listen at (default: 3033)
port: 8989

# Directory where downloaded files will be stored (default: ".")
downloadPath: /home/ren/archive

# [optional] Enable RPC authentication (requires username and password)
require_auth: true
username: my_username
password: my_random_secret

# [optional] The download queue size (default: logical cpu cores)
queue_size: 4 # min. 2

# [optional] Full path to the yt-dlp (default: "yt-dlp")
#downloaderPath: /usr/local/bin/yt-dlp

# [optional] Enable file based logging with rotation (default: false)
#enable_file_logging: false

# [optional] Directory where the log file will be stored (default: ".")
#log_path: .

# [optional] Directory where the session database file will be stored (default: ".")
#session_file_path: .

# [optional] Path where the sqlite database will be created/opened (default: "./local.db")
#local_database_path

# [optional] Path where a custom frontend will be loaded (instead of the embedded one)
#frontend_path: ./web/solid-frontend
```

### Systemd integration
By defining a service file in `/etc/systemd/system/happy-downloader.service` yt-dlp webui can be launched as in background.

```
[Unit]
Description=happy-downloader service file
After=network.target

[Service]
User=some_user
ExecStart=/usr/local/bin/happy-downloader --out /mnt/share/downloads --port 8100

[Install]
WantedBy=multi-user.target
```

```shell
systemctl enable happy-downloader
systemctl start happy-downloader
```
It could be that happy-downloader works correctly when started manually from the console, but with systemd, it does not see the yt-dlp executable, or has issues writing to the database file. One way to fix these issues could be as follows:
```shell
cd
mkdir happy-downloader-workingdir
# optionally move the already existing database file there:
mv local.db happy-downloader-workingdir
nano happy-downloader-workingdir/my.conf
```
The config file format is described above; make sure to include the `downloaderPath` setting (the path can possibly be found by running `which yt-dlp`). For example, one could have:
```
downloadPath: /stuff/media
downloaderPath: /home/your_user/.local/bin/yt-dlp
log_path: /home/your_user/happy-downloader-workingdir
session_file_path: /home/your_user/happy-downloader-workingdir
```
Adjust the Service section in the `/etc/systemd/system/happy-downloader.service` file as follows:
```
[Service]
User=your_user
Group=your_user
WorkingDirectory=/home/your_user/happy-downloader-workingdir
ExecStart=/usr/local/bin/happy-downloader --conf /home/your_user/happy-downloader-workingdir/my.conf
```

## Manual installation
```sh
# the dependencies are: yt-dlp, ffmpeg, nodejs, go, make.

make all
```
## Open-API
Navigate to `/openapi` to see the related swagger.


## Extendable
You dont'like the Material feel?
Want to build your own frontend? We got you covered 🤠

`happy-downloader` now exposes a nice **JSON-RPC 1.0** interface through Websockets and HTTP-POST
It is **planned** to also expose a **gRPC** server.

For more information open an issue on GitHub and I will provide more info ASAP.

## Custom frontend
To load a custom frontend you need to specify its path either in the config file ([see config file](#config-file)) or via flags.

The frontend needs to follow this structure:
```
path/to/my/frontend
├── assets
│   ├── js-chunk-1.js (example)
│   ├── js-chunk-2.js (example)
│   ├── style.css (example)
└── index.html
```

`assets` is where the resources will be loaded.  
`index.html` is the entrypoint.

## Nix
This repo adds support for Nix(OS) in various ways through a `flake-parts` flake. 
For more info, please refer to the [official documentation](https://nixos.org/learn/).

## What happy-downloader is not
`happy-downloader` isn't your ordinary website where to download stuff from the internet, so don't try asking for links of where this is hosted. It's a self hosted platform for a Linux NAS.

## Troubleshooting
-   **It says that it isn't connected.**
    - In some circumstances, you must set the server ip address or hostname in the settings section (gear icon).
-   **The download  doesn't start.**
    - Simply, yt-dlp process takes a lot of time to fire up. (yt-dlp isn't fast especially if you have a lower-end/low-power NAS/server/desktop. Furthermore some yt-dlp builds are slower than others)
